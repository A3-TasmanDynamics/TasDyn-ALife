-- Bank integrity: manual database edits can't leave balances out of step.
--
-- bank_accounts.balance is the real balance; players.<faction>_bank is a
-- cache; bank_transactions is the history. Before this, editing
-- players.civ_bank by hand (psql, the website's database view) changed the
-- cache only -- the phone showed money the ATM couldn't touch. Now:
--   * editing players.civ_bank / cop_bank / medic_bank, or
--     bank_accounts.balance, directly becomes an 'admin_adjustment'
--     transaction for the difference, so balance, cache and history agree;
--   * every such edit is written to staff_log (bank.manual_adjustment,
--     source 'manual') -- website Staff Log and Discord #staff-log;
--   * a transaction's balance_after is always worked out from the real
--     balance, and nothing can take a balance below zero.
-- Edits made by the triggers themselves (pg_trigger_depth() > 1) are the
-- normal cache sync and are left alone. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-17_bank_integrity.sql

BEGIN;

-- Every transaction's balance_after = current balance + amount, never below zero.
CREATE OR REPLACE FUNCTION bank_transactions_balance_after() RETURNS TRIGGER AS $$
DECLARE
    current_balance BIGINT;
BEGIN
    SELECT balance INTO current_balance FROM bank_accounts WHERE id = NEW.account_id FOR UPDATE;
    NEW.balance_after := current_balance + NEW.amount;
    IF NEW.balance_after < 0 THEN
        RAISE EXCEPTION 'Bank balance can''t go below zero (account %, balance %, change %)',
            NEW.account_id, current_balance, NEW.amount;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bank_transactions_balance_after ON bank_transactions;
CREATE TRIGGER trg_bank_transactions_balance_after
    BEFORE INSERT ON bank_transactions
    FOR EACH ROW EXECUTE FUNCTION bank_transactions_balance_after();

-- Records a manual balance change: a ledger entry (which moves the real
-- balance and the cache) and a staff_log line.
CREATE OR REPLACE FUNCTION bank_record_manual_change(p_player BIGINT, p_faction TEXT, p_new BIGINT, p_where TEXT)
RETURNS VOID AS $$
DECLARE
    acct_id BIGINT;
    acct_balance BIGINT;
BEGIN
    IF p_new < 0 THEN
        RAISE EXCEPTION 'Bank balance can''t go below zero (% bank for player %)', p_faction, p_player;
    END IF;
    INSERT INTO bank_accounts (player_id, faction, balance) VALUES (p_player, p_faction, 0)
        ON CONFLICT (player_id, faction) DO NOTHING;
    SELECT id, balance INTO acct_id, acct_balance FROM bank_accounts
        WHERE player_id = p_player AND faction = p_faction FOR UPDATE;
    IF acct_balance = p_new THEN
        RETURN;
    END IF;
    INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo)
        VALUES (acct_id, 'admin_adjustment', p_new - acct_balance, p_new, 'Manual database change (' || p_where || ')');
    INSERT INTO staff_log (target_player_id, action, reason, before_value, after_value, source)
        VALUES (p_player, 'bank.manual_adjustment', 'Bank balance changed directly in the database (' || p_where || ')',
                jsonb_build_object('faction', p_faction, 'balance', acct_balance),
                jsonb_build_object('faction', p_faction, 'balance', p_new), 'manual');
END;
$$ LANGUAGE plpgsql;

-- Direct edits of the cache columns on players.
CREATE OR REPLACE FUNCTION bank_manual_players_edit() RETURNS TRIGGER AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN NEW; -- the cache being synced from bank_accounts
    END IF;
    IF NEW.civ_bank IS DISTINCT FROM OLD.civ_bank THEN
        PERFORM bank_record_manual_change(NEW.id, 'civilian', NEW.civ_bank, 'players.civ_bank');
    END IF;
    IF NEW.cop_bank IS DISTINCT FROM OLD.cop_bank THEN
        PERFORM bank_record_manual_change(NEW.id, 'police', NEW.cop_bank, 'players.cop_bank');
    END IF;
    IF NEW.medic_bank IS DISTINCT FROM OLD.medic_bank THEN
        PERFORM bank_record_manual_change(NEW.id, 'medic', NEW.medic_bank, 'players.medic_bank');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_players_bank_manual_edit ON players;
CREATE TRIGGER trg_players_bank_manual_edit
    AFTER UPDATE OF civ_bank, cop_bank, medic_bank ON players
    FOR EACH ROW EXECUTE FUNCTION bank_manual_players_edit();

-- Direct edits of bank_accounts.balance.
CREATE OR REPLACE FUNCTION bank_manual_account_edit() RETURNS TRIGGER AS $$
BEGIN
    IF pg_trigger_depth() > 1 OR NEW.balance IS NOT DISTINCT FROM OLD.balance THEN
        RETURN NEW; -- set by a transaction (the normal path)
    END IF;
    IF NEW.balance < 0 THEN
        RAISE EXCEPTION 'Bank balance can''t go below zero (account %)', NEW.id;
    END IF;
    -- Put the old balance back, then make the change as a transaction (which
    -- moves the balance to the new value and updates the cache).
    UPDATE bank_accounts SET balance = OLD.balance WHERE id = NEW.id;
    INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo)
        VALUES (NEW.id, 'admin_adjustment', NEW.balance - OLD.balance, NEW.balance,
                'Manual database change (bank_accounts.balance)');
    INSERT INTO staff_log (target_player_id, action, reason, before_value, after_value, source)
        VALUES (NEW.player_id, 'bank.manual_adjustment', 'Bank balance changed directly in the database (bank_accounts.balance)',
                jsonb_build_object('faction', NEW.faction, 'balance', OLD.balance),
                jsonb_build_object('faction', NEW.faction, 'balance', NEW.balance), 'manual');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bank_accounts_manual_edit ON bank_accounts;
CREATE TRIGGER trg_bank_accounts_manual_edit
    AFTER UPDATE OF balance ON bank_accounts
    FOR EACH ROW EXECUTE FUNCTION bank_manual_account_edit();

COMMIT;
