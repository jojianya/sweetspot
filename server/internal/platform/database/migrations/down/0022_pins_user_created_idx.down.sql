-- Rolls back 0022_pins_user_created_idx.sql.
--
-- IF EXISTS so the rollback is safe against a database where the index was
-- never created, and DROP INDEX CONCURRENTLY for the same reason the up
-- migration creates it that way: an exclusive lock on pins on every deploy
-- would block all traffic.
--
-- RollbackLastMigration walks applied migrations newest-first and reverts the
-- first one that ships a rollback script, then deletes its schema_migrations
-- row so a later RunMigrations re-applies it.
--
-- Note 0022 is the newest migration, so this is the script that a plain
-- rollback reaches first. Dropping the index does not lose data: the planner
-- falls back to pins_visible_created_idx with a user_id filter, which is the
-- pre-0022 plan — slower, but correct.

DROP INDEX CONCURRENTLY IF EXISTS pins_user_created_idx;
