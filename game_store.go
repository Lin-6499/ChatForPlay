package main

import "database/sql"

func migrateGames(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS service_accounts (user_id INTEGER PRIMARY KEY REFERENCES users(id));
	CREATE TABLE IF NOT EXISTS campaigns (
	  id INTEGER PRIMARY KEY, conversation_id INTEGER NOT NULL UNIQUE REFERENCES conversations(id),
	  owner_id INTEGER NOT NULL REFERENCES users(id), bot_id INTEGER NOT NULL REFERENCES users(id),
	  title TEXT NOT NULL, ruleset TEXT NOT NULL, house_rules TEXT NOT NULL DEFAULT '',
	  scene TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', phase TEXT NOT NULL DEFAULT 'paused',
	  mode TEXT NOT NULL DEFAULT 'manual', auto_respond INTEGER NOT NULL DEFAULT 1,
	  version INTEGER NOT NULL DEFAULT 1, active_turn INTEGER NOT NULL DEFAULT 0,
	  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS campaign_notes (
	  campaign_id INTEGER PRIMARY KEY REFERENCES campaigns(id), content TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS characters (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id), user_id INTEGER NOT NULL REFERENCES users(id),
	  name TEXT NOT NULL, role TEXT NOT NULL, level INTEGER NOT NULL, hp INTEGER NOT NULL, max_hp INTEGER NOT NULL,
	  ac INTEGER NOT NULL, abilities TEXT NOT NULL, conditions TEXT NOT NULL DEFAULT '',
	  inventory TEXT NOT NULL DEFAULT '', resources TEXT NOT NULL DEFAULT '', background TEXT NOT NULL DEFAULT '',
	  private_notes TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1,
	  UNIQUE(campaign_id,user_id)
	);
	CREATE TABLE IF NOT EXISTS campaign_events (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id), actor_id INTEGER NOT NULL REFERENCES users(id),
	  kind TEXT NOT NULL, visibility TEXT NOT NULL DEFAULT 'public', recipient_id INTEGER NOT NULL DEFAULT 0,
	  content TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS campaign_events_order ON campaign_events(campaign_id,id);
	CREATE TABLE IF NOT EXISTS campaign_lore (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id),
	  kind TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL,
	  visibility TEXT NOT NULL, status TEXT NOT NULL, in_context INTEGER NOT NULL,
	  version INTEGER NOT NULL, updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS campaign_lore_list ON campaign_lore(campaign_id,status,id);
	CREATE TABLE IF NOT EXISTS campaign_lore_history (
	  entry_id INTEGER NOT NULL REFERENCES campaign_lore(id), version INTEGER NOT NULL,
	  actor_id INTEGER NOT NULL, document TEXT NOT NULL, created_at INTEGER NOT NULL,
	  PRIMARY KEY(entry_id,version)
	);
	CREATE TABLE IF NOT EXISTS campaign_operations (
	  campaign_id INTEGER NOT NULL REFERENCES campaigns(id), actor TEXT NOT NULL, client_id TEXT NOT NULL,
	  fingerprint TEXT NOT NULL, response TEXT NOT NULL, PRIMARY KEY(campaign_id,actor,client_id)
	);
	CREATE TABLE IF NOT EXISTS campaign_turns (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id),
	  event_id INTEGER NOT NULL UNIQUE REFERENCES campaign_events(id), kind TEXT NOT NULL DEFAULT 'action',
	  status TEXT NOT NULL DEFAULT 'queued', claimant TEXT NOT NULL DEFAULT '', lease TEXT NOT NULL DEFAULT '',
	  base_version INTEGER NOT NULL DEFAULT 0, started_at INTEGER NOT NULL DEFAULT 0,
	  error TEXT NOT NULL DEFAULT '', result TEXT NOT NULL DEFAULT '{}'
	);
	CREATE INDEX IF NOT EXISTS campaign_turns_queue ON campaign_turns(campaign_id,status,id);
	CREATE TABLE IF NOT EXISTS campaign_drafts (
	  campaign_id INTEGER PRIMARY KEY REFERENCES campaigns(id),
	  turn_id INTEGER NOT NULL, lease TEXT NOT NULL, seq INTEGER NOT NULL,
	  narration TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS campaign_turn_steps (
	  turn_id INTEGER NOT NULL REFERENCES campaign_turns(id), step INTEGER NOT NULL,
	  assistant TEXT NOT NULL, tool_result TEXT NOT NULL DEFAULT '',
	  PRIMARY KEY(turn_id,step)
	);
	CREATE TABLE IF NOT EXISTS campaign_proposals (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id), author_id INTEGER NOT NULL REFERENCES users(id),
	  kind TEXT NOT NULL, content TEXT NOT NULL, character_id INTEGER NOT NULL DEFAULT 0,
	  status TEXT NOT NULL DEFAULT 'pending', version INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS campaign_tokens (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id),
	  token_hash TEXT NOT NULL UNIQUE, label TEXT NOT NULL, secrets INTEGER NOT NULL DEFAULT 0,
	  expires_at INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS game_ai_settings (
	  id INTEGER PRIMARY KEY CHECK(id=1), base_url TEXT NOT NULL, model TEXT NOT NULL,
	  key_cipher TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS campaign_checks (
	  id INTEGER PRIMARY KEY, campaign_id INTEGER NOT NULL REFERENCES campaigns(id),
	  event_id INTEGER NOT NULL REFERENCES campaign_events(id), character_id INTEGER NOT NULL REFERENCES characters(id),
	  turn_id INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, document TEXT NOT NULL,
	  UNIQUE(campaign_id,event_id)
	);
	CREATE INDEX IF NOT EXISTS campaign_checks_pending ON campaign_checks(campaign_id,character_id,status);
	CREATE TABLE IF NOT EXISTS campaign_replies (
	  campaign_id INTEGER NOT NULL REFERENCES campaigns(id),
	  prompt_event_id INTEGER NOT NULL REFERENCES campaign_events(id),
	  user_id INTEGER NOT NULL REFERENCES users(id),
	  event_id INTEGER NOT NULL REFERENCES campaign_events(id),
	  PRIMARY KEY(campaign_id,prompt_event_id,user_id)
	);
	PRAGMA user_version=6;`)
	return err
}
