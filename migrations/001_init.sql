-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════════════
-- EXTENSIONS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ═══════════════════════════════════════════════════════════════════════════
-- CURRENCIES
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE currencies (
    code            CHAR(3) PRIMARY KEY,
    name            TEXT NOT NULL,
    symbol          TEXT NOT NULL,
    decimal_digits  SMALLINT NOT NULL CHECK (decimal_digits BETWEEN 0 AND 4),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO currencies (code, name, symbol, decimal_digits) VALUES
    ('USD', 'US Dollar',        '$',    2),
    ('EUR', 'Euro',             '€',    2),
    ('GBP', 'British Pound',    '£',    2),
    ('JPY', 'Japanese Yen',     '¥',    0),
    ('CNY', 'Chinese Yuan',     '¥',    2),
    ('BRL', 'Brazilian Real',   'R$',   2),
    ('INR', 'Indian Rupee',     '₹',    2),
    ('MAD', 'Moroccan Dirham',  'د.م.', 2),
    ('NGN', 'Nigerian Naira',   '₦',    2),
    ('KES', 'Kenyan Shilling',  'KSh',  2);

-- ═══════════════════════════════════════════════════════════════════════════
-- USERS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               CITEXT UNIQUE NOT NULL,
    password_hash       TEXT NOT NULL,
    username            CITEXT UNIQUE NOT NULL,
    preferred_currency  CHAR(3) NOT NULL REFERENCES currencies(code),
    is_admin            BOOLEAN NOT NULL DEFAULT FALSE,
    is_system           BOOLEAN NOT NULL DEFAULT FALSE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    kyc_status          TEXT NOT NULL DEFAULT 'not_required'
                          CHECK (kyc_status IN ('not_required','pending','verified','rejected')),
    kyc_override        BOOLEAN NOT NULL DEFAULT FALSE,
    disabled_at         TIMESTAMPTZ NULL,
    disable_reason      TEXT NULL,
    email_verified_at   TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9_]{3,32}$')
);

CREATE INDEX users_admin_active_idx
    ON users(is_active, created_at DESC)
    WHERE is_admin = FALSE AND is_system = FALSE;

-- System user (cannot log in — invalid bcrypt hash; no one can authenticate as it)
INSERT INTO users (
    email, password_hash, username, preferred_currency, is_system, is_admin, is_active
) VALUES (
    'system@sindri.local', 'x', 'system', 'USD', TRUE, FALSE, TRUE
) ON CONFLICT (email) DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════
-- USER PROFILES
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE user_profiles (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    bio          TEXT NOT NULL DEFAULT '',
    skills       TEXT[] NOT NULL DEFAULT '{}',
    interests    TEXT[] NOT NULL DEFAULT '{}',
    avatar_url   TEXT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ═══════════════════════════════════════════════════════════════════════════
-- CATEGORIES (self-referencing tree, one level of nesting enforced in service)
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]{2,64}$'),
    name        TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description TEXT NOT NULL DEFAULT '',
    sort_order  INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    parent_id   UUID NULL REFERENCES categories(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT categories_no_self_parent CHECK (id <> parent_id)
);

CREATE INDEX categories_active_idx ON categories(is_active, sort_order, name);
CREATE INDEX categories_parent_idx ON categories(parent_id, sort_order, name)
    WHERE parent_id IS NOT NULL;

-- Seed top-level + subcategories
INSERT INTO categories (slug, name, description, sort_order) VALUES
    ('code',         'Code',            'Source code, scripts, snippets, tools for developers', 10),
    ('design',       'Design',          'UI kits, icons, illustrations, fonts, mockups',          20),
    ('audio',        'Audio',           'Sample packs, presets, loops, plugins, sound design',    30),
    ('video',        'Video',           'Presets, transitions, LUTs, motion graphics',            40),
    ('3d',           '3D Assets',       'Models, textures, materials, rigged characters',         50),
    ('ebooks',       'Ebooks & Guides', 'Technical books, tutorials, cheatsheets, zines',         60),
    ('music',        'Music',           'Tracks, beats, licenses, custom compositions',           70),
    ('services',     'Services',        'Freelance gigs: code review, design, dev, consulting',   80),
    ('collectibles', 'Collectibles',    'Vintage tech, signed merch, rare items',                 90)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('code-perl',         'Perl',                10),
    ('code-golang',       'Golang',              20),
    ('code-rust',         'Rust',                30),
    ('code-python',       'Python',              40),
    ('code-c-cpp',        'C/C++',               50),
    ('code-php',          'Php',                 60),
    ('code-java',         'Java',                70),
    ('code-js',           'JavaScript & TS',     80),
    ('code-shell',        'Shell & CLI tools',   90),
    ('code-devops',       'DevOps & Infra',     100),
    ('code-security',     'Security & Hacking', 110),
    ('code-blockchain',   'Blockchain',         120),
    ('code-gamedev',      'Game Development',   130),
    ('code-mobile',       'Mobile',             140)
) AS v(slug, name, sort_order)
CROSS JOIN categories c
WHERE c.slug = 'code'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('design-ui',         'UI Kits',            10),
    ('design-icons',      'Icons',              20),
    ('design-fonts',      'Fonts',              30),
    ('design-illust',     'Illustrations',      40),
    ('design-mockup',     'Mockups',            50),
    ('design-3d',         '3D Design',          60)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'design'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('audio-samples',     'Sample Packs',       10),
    ('audio-presets',     'Presets',            20),
    ('audio-loops',       'Loops',              30),
    ('audio-plugins',     'Plugins & VSTs',     40),
    ('audio-foley',       'Foley & SFX',        50)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'audio'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('video-presets',     'Presets',            10),
    ('video-transitions', 'Transitions',        20),
    ('video-luts',        'LUTs & Color',       30),
    ('video-motion',      'Motion Graphics',    40)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'video'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('3d-models',         'Models',             10),
    ('3d-textures',       'Textures',           20),
    ('3d-characters',     'Characters',         30),
    ('3d-environments',   'Environments',       40)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = '3d'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('ebooks-programming','Programming',        10),
    ('ebooks-sysadmin',   'Sysadmin & DevOps',  20),
    ('ebooks-security',   'Security',           30),
    ('ebooks-design',     'Design',             40),
    ('ebooks-business',   'Business & Startup', 50)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'ebooks'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('music-beats',       'Beats',              10),
    ('music-ambient',     'Ambient & Cinematic',20),
    ('music-synth',       'Synth & Electronic', 30),
    ('music-rap',         'Rap & Hip-Hop',      40),
    ('music-rock',        'Rock & Metal',       50),
    ('music-classical',   'Classical',          60),
    ('music-jazz',        'Jazz & Blues',       70),
    ('music-game',        'Game Soundtracks',   80)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'music'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('services-dev',        'Development',           10),
    ('services-design',     'Design',                20),
    ('services-code-review','Code Review',           30),
    ('services-consulting', 'Consulting',            40),
    ('services-translation','Technical Translation', 50)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'services'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories (slug, name, parent_id, sort_order)
SELECT v.slug, v.name, c.id, v.sort_order
FROM (VALUES
    ('collect-vintage',     'Vintage Tech',      10),
    ('collect-games',       'Vintage Games',     20),
    ('collect-merch',       'Merch & Apparel',   30),
    ('collect-books',       'Rare Books',        40)
) AS v(slug, name, sort_order)
CROSS JOIN categories c WHERE c.slug = 'collectibles'
ON CONFLICT (slug) DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════
-- PRODUCTS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE products (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id     UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title         TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description   TEXT NOT NULL DEFAULT '',
    price         BIGINT NOT NULL CHECK (price > 0),
    currency_code CHAR(3) NOT NULL REFERENCES currencies(code),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    category_id   UUID NULL REFERENCES categories(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX products_seller_idx   ON products(seller_id, created_at DESC);
CREATE INDEX products_active_idx   ON products(is_active, created_at DESC) WHERE is_active;
CREATE INDEX products_currency_idx ON products(currency_code);
CREATE INDEX products_category_idx ON products(category_id, created_at DESC)
    WHERE category_id IS NOT NULL;

-- ═══════════════════════════════════════════════════════════════════════════
-- WALLETS + LEDGER
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE wallets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    currency_code CHAR(3) NOT NULL REFERENCES currencies(code),
    available     BIGINT NOT NULL DEFAULT 0 CHECK (available >= 0),
    held          BIGINT NOT NULL DEFAULT 0 CHECK (held >= 0),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, currency_code)
);

CREATE INDEX wallets_user_idx ON wallets(user_id);

-- ═══════════════════════════════════════════════════════════════════════════
-- ORDERS + INVOICES + DISPUTES
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE orders (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id           UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_id          UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id         UUID NULL REFERENCES products(id) ON DELETE RESTRICT,
    currency_code      CHAR(3) NOT NULL REFERENCES currencies(code),
    amount             BIGINT NOT NULL CHECK (amount > 0),
    commission_amount  BIGINT NOT NULL CHECK (commission_amount >= 0),
    commission_percent INT NOT NULL CHECK (commission_percent BETWEEN 0 AND 100),
    status             TEXT NOT NULL CHECK (status IN (
                         'pending','held','disputed','completed','refunded','cancelled'
                       )),
    auto_release_at    TIMESTAMPTZ NULL,
    released_at        TIMESTAMPTZ NULL,
    released_by        TEXT NULL CHECK (released_by IN ('buyer','auto','admin')),
    invoice_title       TEXT NULL,
    invoice_description TEXT NULL,
    cancel_reason      TEXT NULL,
    cancelled_by       UUID NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (buyer_id <> seller_id),
    CHECK (commission_amount <= amount)
);

CREATE INDEX orders_buyer_idx           ON orders(buyer_id, created_at DESC, id DESC);
CREATE INDEX orders_seller_idx          ON orders(seller_id, created_at DESC, id DESC);
CREATE INDEX orders_stale_held_idx      ON orders(auto_release_at) WHERE status = 'held';
CREATE INDEX orders_pending_invoice_idx ON orders(buyer_id, created_at DESC) WHERE status = 'pending';
CREATE INDEX orders_status_created_idx  ON orders(status, created_at DESC);

CREATE TABLE disputes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    opened_by   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason      TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 2000),
    resolution  TEXT NULL CHECK (resolution IS NULL OR resolution IN ('refunded','released')),
    resolved_by UUID NULL REFERENCES users(id) ON DELETE RESTRICT,
    resolved_at TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT disputes_resolution_consistency CHECK (
        (resolution IS NULL AND resolved_by IS NULL AND resolved_at IS NULL) OR
        (resolution IS NOT NULL AND resolved_by IS NOT NULL AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX disputes_order_idx        ON disputes(order_id);
CREATE INDEX disputes_unresolved_idx   ON disputes(created_at DESC) WHERE resolution IS NULL;

-- ═══════════════════════════════════════════════════════════════════════════
-- WALLET TRANSACTIONS (ledger) — now with order_id FK
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE wallet_transactions (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id              UUID NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    currency_code          CHAR(3) NOT NULL REFERENCES currencies(code),
    type                   TEXT NOT NULL CHECK (type IN (
                             'deposit','withdraw','escrow_hold',
                             'escrow_release','escrow_refund',
                             'commission','admin_adjustment'
                           )),
    direction              TEXT NOT NULL CHECK (direction IN ('credit','debit')),
    amount                 BIGINT NOT NULL CHECK (amount > 0),
    balance_after          BIGINT NOT NULL,
    order_id               UUID NULL REFERENCES orders(id) ON DELETE RESTRICT,
    counterparty_wallet_id UUID NULL REFERENCES wallets(id),
    status                 TEXT NOT NULL DEFAULT 'completed'
                             CHECK (status IN ('pending','completed','failed','reversed')),
    idempotency_key        TEXT UNIQUE NULL,
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX wtx_wallet_created_idx ON wallet_transactions(wallet_id, created_at DESC);
CREATE INDEX wtx_user_created_idx   ON wallet_transactions(user_id, created_at DESC);
CREATE INDEX wtx_type_created_idx   ON wallet_transactions(type, created_at DESC);
CREATE INDEX wtx_order_id_idx       ON wallet_transactions(order_id)
    WHERE order_id IS NOT NULL;
CREATE INDEX wtx_user_history_idx   ON wallet_transactions(user_id, created_at DESC, id DESC);

-- ═══════════════════════════════════════════════════════════════════════════
-- WITHDRAWAL REQUESTS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE withdrawal_requests (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    currency_code    CHAR(3) NOT NULL REFERENCES currencies(code),
    amount           BIGINT NOT NULL CHECK (amount > 0),
    status           TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','approved','rejected')),
    rejection_reason TEXT NULL,
    reviewed_by      UUID NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_at      TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT withdrawal_review_consistency CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR
        (status IN ('approved','rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE INDEX withdrawal_user_idx    ON withdrawal_requests(user_id, created_at DESC);
CREATE INDEX withdrawal_pending_idx ON withdrawal_requests(created_at ASC) WHERE status = 'pending';

-- ═══════════════════════════════════════════════════════════════════════════
-- REVIEWS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    buyer_id    UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id  UUID NULL REFERENCES products(id) ON DELETE SET NULL,
    rating      SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     TEXT NOT NULL DEFAULT '' CHECK (length(comment) <= 2000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reviews_seller_idx  ON reviews(seller_id, created_at DESC);
CREATE INDEX reviews_product_idx ON reviews(product_id, created_at DESC)
    WHERE product_id IS NOT NULL;

-- ═══════════════════════════════════════════════════════════════════════════
-- NOTIFICATIONS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    title      TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    body       TEXT NOT NULL DEFAULT '',
    metadata   JSONB NOT NULL DEFAULT '{}',
    read_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_idx   ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON notifications(user_id, created_at DESC)
    WHERE read_at IS NULL;

-- ═══════════════════════════════════════════════════════════════════════════
-- CHAT (encrypted at rest — no plaintext column ever existed)
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE chat_conversations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        TEXT NOT NULL CHECK (kind IN ('dm','group')),
    title       TEXT NULL CHECK (title IS NULL OR length(title) BETWEEN 1 AND 200),
    dm_key      TEXT NULL,
    created_by  UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'dm'    AND title IS NULL AND dm_key IS NOT NULL) OR
        (kind = 'group' AND title IS NOT NULL AND dm_key IS NULL)
    )
);

CREATE UNIQUE INDEX chat_dm_unique ON chat_conversations(dm_key) WHERE kind = 'dm';

CREATE TABLE chat_members (
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at    TIMESTAMPTZ NULL,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX chat_members_user_idx ON chat_members(user_id, joined_at DESC);

CREATE TABLE chat_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    ciphertext      BYTEA NOT NULL,
    nonce           BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chat_messages_conv_idx ON chat_messages(conversation_id, created_at DESC, id DESC);

-- ═══════════════════════════════════════════════════════════════════════════
-- TICKETS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE tickets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject     TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 300),
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','answered','closed')),
    priority    TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tickets_user_idx   ON tickets(user_id, created_at DESC);
CREATE INDEX tickets_status_idx ON tickets(status, created_at DESC);

CREATE TABLE ticket_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_staff    BOOLEAN NOT NULL DEFAULT FALSE,
    body        TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 10000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ticket_messages_ticket_idx ON ticket_messages(ticket_id, created_at);

-- ═══════════════════════════════════════════════════════════════════════════
-- WARNINGS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE warnings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issued_by       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason          TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 2000),
    severity        TEXT NOT NULL DEFAULT 'info' CHECK (severity IN ('info','warning','severe')),
    acknowledged_at TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX warnings_user_idx   ON warnings(user_id, created_at DESC);
CREATE INDEX warnings_unack_idx  ON warnings(user_id, created_at DESC)
    WHERE acknowledged_at IS NULL;

-- ═══════════════════════════════════════════════════════════════════════════
-- PLATFORM SETTINGS
-- ═══════════════════════════════════════════════════════════════════════════
CREATE TABLE platform_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID NULL REFERENCES users(id)
);

INSERT INTO platform_settings (key, value) VALUES
    ('kyc_required_for_deposit',  'false'::jsonb),
    ('kyc_required_for_withdraw', 'false'::jsonb),
    ('commission_percent',        '5'::jsonb),
    ('escrow_auto_release_days',  '3'::jsonb);

-- ═══════════════════════════════════════════════════════════════════════════
-- SEARCH OPTIMIZATIONS (pg_trgm)
-- ═══════════════════════════════════════════════════════════════════════════
CREATE INDEX users_username_trgm_idx    ON users USING GIN (username gin_trgm_ops);
CREATE INDEX profiles_bio_trgm_idx      ON user_profiles USING GIN (bio gin_trgm_ops);
CREATE INDEX products_title_trgm_idx    ON products USING GIN (title gin_trgm_ops);
CREATE INDEX products_desc_trgm_idx     ON products USING GIN (description gin_trgm_ops);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS warnings CASCADE;
DROP TABLE IF EXISTS ticket_messages CASCADE;
DROP TABLE IF EXISTS tickets CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS chat_members CASCADE;
DROP TABLE IF EXISTS chat_conversations CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS reviews CASCADE;
DROP TABLE IF EXISTS withdrawal_requests CASCADE;
DROP TABLE IF EXISTS wallet_transactions CASCADE;
DROP TABLE IF EXISTS disputes CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS wallets CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS user_profiles CASCADE;
DROP TABLE IF EXISTS platform_settings CASCADE;
DELETE FROM users WHERE is_system = TRUE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS currencies CASCADE;
-- +goose StatementEnd
