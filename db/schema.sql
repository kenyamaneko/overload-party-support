CREATE SCHEMA IF NOT EXISTS support;

-- お知らせ本体 (言語非依存の属性)
-- title / body は言語別に announcement_translations で保持する。
CREATE TABLE IF NOT EXISTS support.announcements (
    announcement_id BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type            VARCHAR(20) NOT NULL,                     -- info / maintenance / event / update
    published_at    TIMESTAMPTZ,                              -- NULL: 下書き, 未来日時: 予約公開, <=now: 公開中
    expires_at      TIMESTAMPTZ,                              -- NULL: 無期限公開
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),       -- レコード作成日時
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()        -- 最終更新日時 (trigger で自動更新)
);

-- お知らせの言語別コンテンツ。
-- 親お知らせ 1 行に対し、対応言語ごとに 1 行を持つ (MVP は ja / en)。
CREATE TABLE IF NOT EXISTS support.announcement_translations (
    announcement_id BIGINT       NOT NULL REFERENCES support.announcements(announcement_id) ON DELETE CASCADE,
    lang            VARCHAR(10)  NOT NULL,                    -- ja / en 等 (許容値は support 側の定数)
    title           VARCHAR(200) NOT NULL,
    body            TEXT         NOT NULL,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),      -- trigger で自動更新
    PRIMARY KEY (announcement_id, lang)
);

-- 問い合わせ。status / internal_note も同一行に持つ。
CREATE TABLE IF NOT EXISTS support.inquiries (
    inquiry_id    BIGINT        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title         VARCHAR(100)  NOT NULL,
    body          VARCHAR(4000) NOT NULL,
    reply_email   VARCHAR(255)  NOT NULL,
    status        VARCHAR(20)   NOT NULL,                     -- new / in_progress / closed
    internal_note TEXT,                                       -- 運営向けメモ (プレイヤーは参照不可)
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT inquiries_status_check CHECK (status IN ('new', 'in_progress', 'closed'))
);

-- 公開 API の一覧クエリ用: 下書き (published_at IS NULL) は索引に載せない
CREATE INDEX IF NOT EXISTS idx_announcements_published
    ON support.announcements (published_at DESC, announcement_id DESC)
    WHERE published_at IS NOT NULL;

-- ステータス別の一覧抽出用: status フィルタ + updated_at DESC
CREATE INDEX IF NOT EXISTS idx_inquiries_status_updated
    ON support.inquiries (status, updated_at DESC);

-- 任意の UPDATE で updated_at を自動更新する trigger。
CREATE OR REPLACE FUNCTION support.touch_updated_at()
    RETURNS TRIGGER
    LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_announcements_touch_updated_at
    BEFORE UPDATE ON support.announcements
    FOR EACH ROW
    EXECUTE FUNCTION support.touch_updated_at();

CREATE TRIGGER trg_announcement_translations_touch_updated_at
    BEFORE UPDATE ON support.announcement_translations
    FOR EACH ROW
    EXECUTE FUNCTION support.touch_updated_at();

CREATE TRIGGER trg_inquiries_touch_updated_at
    BEFORE UPDATE ON support.inquiries
    FOR EACH ROW
    EXECUTE FUNCTION support.touch_updated_at();
