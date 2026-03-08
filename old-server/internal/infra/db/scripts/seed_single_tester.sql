-- Create a temporary table instead of a CTE
CREATE TEMPORARY TABLE constants (
  account_id uuid,
  user_id uuid,
  role_id uuid,
  permission_id uuid,
  app_id uuid,
  session_id uuid,
  prompt_id uuid
);

-- Insert the constant values
INSERT INTO constants VALUES (
  '00000000-0000-0000-0000-000000000001'::uuid,
  '00000000-0000-0000-0000-000000000002'::uuid,
  '00000000-0000-0000-0000-000000000003'::uuid,
  '00000000-0000-0000-0000-000000000004'::uuid,
  '00000000-0000-0000-0000-000000000005'::uuid,
  '00000000-0000-0000-0000-000000000006'::uuid,
  '00000000-0000-0000-0000-000000000007'::uuid
);

-- 👤 Account
INSERT INTO accounts (id, name)
SELECT account_id, 'Acme Corp' FROM constants;

-- 👥 User (add role)
INSERT INTO users (id, account_id, email, role)
SELECT user_id, account_id, 'demo@acme.com', 'owner' FROM constants;

-- 🧑‍⚖️ Role (add access_level)
INSERT INTO roles (id, account_id, name, access_level)
SELECT role_id, account_id, 'admin', 100 FROM constants;

-- 🔗 Role Assignment (fix column order)
INSERT INTO role_users (user_id, role_id)
SELECT user_id, role_id FROM constants;

-- 🔐 Permission (add required columns)
INSERT INTO permissions (id, account_id, entity, actions, description)
SELECT permission_id, account_id, 'users', ARRAY['retrieve']::actions[], 'Can read session data' FROM constants;

-- 🔗 Permission-Role Mapping (fix column order)
INSERT INTO permission_roles (role_id, permission_id)
SELECT role_id, permission_id FROM constants;

-- 📱 App
INSERT INTO apps (id, account_id, name, description, api_key, config)
SELECT app_id, account_id, 'Voice Notes App', 'Test app for voice-to-function', '2ck9l0gW7glVtwmoVWnKezzFzA6Ez1qAuAYH2RO4hcQ=', 
    '{"AllowedOrigins": ["http://localhost:3000", "https://localhost:3000"], "EnabledSchemas": [], "PreferredLLM": "gemini-2.0-flash"}'::json 
FROM constants;

-- 📶 Session
INSERT INTO sessions (id, app_id, created_at)
SELECT session_id, app_id, NOW() FROM constants;

-- 🧠 Prompt (add required columns, use text for content)
INSERT INTO prompts (id, app_id, session_id, content, checksum)
SELECT prompt_id, app_id, session_id, 'Summarize user input into actions.', 'dummychecksum' FROM constants;

-- 🔗 Link prompt to session
INSERT INTO session_prompts (session_id, prompt_id)
SELECT session_id, prompt_id FROM constants;

-- 📊 Usage Logs
INSERT INTO usage_logs (id, session_id, app_id, account_id, type, metric, logged_at)
SELECT 
  gen_random_uuid(), 
  session_id, 
  app_id, 
  account_id, 
  'llm', 
  '{"tokens": 120, "cost": 0.001}'::json,
  NOW()
FROM constants;

-- 📈 Session Totals (add updated_at)
INSERT INTO session_usage_totals (
  session_id, account_id, app_id, 
  audio_seconds, prompt_tokens, completion_tokens, 
  cpu_active_seconds, cpu_idle_seconds, total_cost, updated_at
)
SELECT 
  session_id, account_id, app_id, 
  2.5, 50, 70, 
  0.2, 0.1, 0.003, NOW()
FROM constants;

-- Clean up the temporary table when done
DROP TABLE constants;