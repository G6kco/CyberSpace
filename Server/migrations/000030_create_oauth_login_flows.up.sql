CREATE TABLE oauth_login_flows (
    state_hash BINARY(32) NOT NULL,
    browser_hash BINARY(32) NOT NULL,
    nonce VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    pkce_verifier VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    consumed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,

    PRIMARY KEY (state_hash),
    INDEX idx_oauth_login_flows_expires_at (expires_at)
) ENGINE = InnoDB;