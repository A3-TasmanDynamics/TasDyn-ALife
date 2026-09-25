// Package bank implements the website's money-moving writes against
// bank_accounts/bank_transactions -- the same tables and the same
// authoritative-ledger design (bank_accounts.balance is truth,
// players.*_bank is a trigger-synced cache) the eventual in-game banking
// feature will use, per docs/WEBSITE.md §5.
//
// Verified safe to build before building it: fn_save.sqf's field allowlist
// (src/ALife.Altis/functions/data/fn_save.sqf) does not include
// civ_bank/cop_bank/medic_bank at all -- the game can LOAD *_bank (it's in
// the load response) but has no path to SAVE it, and the C++ extension's
// own db.cpp confirms the same (bank_accounts is only ever seeded to 0 at
// player creation and read at load, never written by "save"). So there is
// no live in-game write path this package's transfers could race against
// or be silently overwritten by -- the concern flagged in WEBSITE.md §5 was
// real to check, and checking it out ruled it out, not assumed away.
package bank

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/playerlookup"
)

var (
	ErrInvalidAmount      = errors.New("bank: amount must be positive")
	ErrInvalidFaction     = errors.New("bank: faction must be civilian, police, or medic")
	ErrSameAccount        = errors.New("bank: source and destination are the same account")
	ErrInsufficientFunds  = errors.New("bank: insufficient funds")
	ErrDuplicateRequest   = errors.New("bank: this transfer was already processed")
	ErrRecipientNotFound  = errors.New("bank: no player found with that name")
	ErrRecipientAmbiguous = errors.New("bank: more than one player has that name -- ask them for their exact name")
	ErrSelfTransfer       = errors.New("bank: use \"move between my accounts\" to transfer to yourself")
)

func validFaction(f string) bool {
	return f == "civilian" || f == "police" || f == "medic"
}

// TransferOwnAccounts moves money between two of the same player's own
// faction bank accounts (e.g. civilian -> police).
func TransferOwnAccounts(ctx context.Context, pool *pgxpool.Pool, playerID int64, fromFaction, toFaction string, amountCents int64, requestToken string) error {
	if !validFaction(fromFaction) || !validFaction(toFaction) {
		return ErrInvalidFaction
	}
	if fromFaction == toFaction {
		return ErrSameAccount
	}

	fromAccountID, err := accountID(ctx, pool, playerID, fromFaction)
	if err != nil {
		return err
	}
	toAccountID, err := accountID(ctx, pool, playerID, toFaction)
	if err != nil {
		return err
	}

	return moveMoney(ctx, pool, fromAccountID, toAccountID, amountCents, requestToken)
}

// TransferToPlayer moves money from the sender's fromFaction account to a
// DIFFERENT player's toFaction account, identified by exact (case-
// insensitive) name match. Ambiguous or missing names are rejected rather
// than guessed at -- see the package doc for why names, not a numeric ID,
// are the input here: this is what a player can actually see/ask another
// player for, and a wrong guess routing real money to the wrong account is
// exactly the class of mistake worth an extra rejection to prevent.
func TransferToPlayer(ctx context.Context, pool *pgxpool.Pool, senderPlayerID int64, fromFaction string, recipientName string, toFaction string, amountCents int64, requestToken string) error {
	if !validFaction(fromFaction) || !validFaction(toFaction) {
		return ErrInvalidFaction
	}

	recipientID, err := playerlookup.ByExactName(ctx, pool, recipientName)
	switch {
	case err == playerlookup.ErrNotFound:
		return ErrRecipientNotFound
	case err == playerlookup.ErrAmbiguous:
		return ErrRecipientAmbiguous
	case err != nil:
		return err
	}
	if recipientID == senderPlayerID {
		return ErrSelfTransfer
	}

	fromAccountID, err := accountID(ctx, pool, senderPlayerID, fromFaction)
	if err != nil {
		return err
	}
	toAccountID, err := accountID(ctx, pool, recipientID, toFaction)
	if err != nil {
		return err
	}

	return moveMoney(ctx, pool, fromAccountID, toAccountID, amountCents, requestToken)
}

func accountID(ctx context.Context, pool *pgxpool.Pool, playerID int64, faction string) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `SELECT id FROM bank_accounts WHERE player_id = $1 AND faction = $2`, playerID, faction).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("bank: no %s bank account for player %d (every player should have one per faction, seeded at creation)", faction, playerID)
	}
	return id, err
}

// moveMoney is the one place balances actually change -- both
// TransferOwnAccounts and TransferToPlayer resolve to a pair of account
// IDs and call this. Locks both accounts (in a fixed ascending-ID order,
// so two concurrent transfers touching the same two accounts in opposite
// directions can't deadlock each other) before reading balances, so a
// concurrent transfer against the same account can't be read-then-
// overwritten -- the classic double-spend race this row-level locking
// exists to close.
func moveMoney(ctx context.Context, pool *pgxpool.Pool, fromAccountID, toAccountID int64, amountCents int64, requestToken string) error {
	if amountCents <= 0 {
		return ErrInvalidAmount
	}
	if fromAccountID == toAccountID {
		return ErrSameAccount
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	first, second := fromAccountID, toAccountID
	if second < first {
		first, second = second, first
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM bank_accounts WHERE id = $1 FOR UPDATE`, first); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM bank_accounts WHERE id = $1 FOR UPDATE`, second); err != nil {
		return err
	}

	var fromBalance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM bank_accounts WHERE id = $1`, fromAccountID).Scan(&fromBalance); err != nil {
		return err
	}
	if fromBalance < amountCents {
		return ErrInsufficientFunds
	}
	var toBalance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM bank_accounts WHERE id = $1`, toAccountID).Scan(&toBalance); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO bank_transactions (account_id, type, amount, balance_after, related_account_id, request_token)
		VALUES ($1, 'transfer_out', $2, $3, $4, $5)
	`, fromAccountID, -amountCents, fromBalance-amountCents, toAccountID, requestToken)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateRequest
		}
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO bank_transactions (account_id, type, amount, balance_after, related_account_id, request_token)
		VALUES ($1, 'transfer_in', $2, $3, $4, $5)
	`, toAccountID, amountCents, toBalance+amountCents, fromAccountID, requestToken)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateRequest
		}
		return err
	}

	return tx.Commit(ctx)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
