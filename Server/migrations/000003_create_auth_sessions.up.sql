CREATE TABLE auth_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,

    token_hash BINARY(32) NOT NULL,

    ip_address VARBINARY(16) NULL,
    user_agent VARCHAR(512) NULL,

    last_seen_at DATETIME(6) NULL,
    expires_at DATETIME(6) NOT NULL,
    revoked_at DATETIME(6) NULL,
    revocation_reason VARCHAR(255) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_auth_sessions_token_hash
        UNIQUE (token_hash),

    CONSTRAINT chk_auth_sessions_expiry
        CHECK (expires_at > created_at),

    CONSTRAINT chk_auth_sessions_revoked_at
        CHECK (
            revoked_at IS NULL
            OR revoked_at >= created_at
        ),

    CONSTRAINT chk_auth_sessions_revocation_reason
        CHECK (
            revocation_reason IS NULL
            OR revoked_at IS NOT NULL
        ),

    CONSTRAINT fk_auth_sessions_user
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_auth_sessions_user_active (
        user_id,
        revoked_at,
        expires_at
    ),

    INDEX idx_auth_sessions_expires_at (
        expires_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;