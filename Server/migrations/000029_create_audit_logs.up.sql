CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    actor_user_id BIGINT UNSIGNED NULL,

    action VARCHAR(100) NOT NULL,
    outcome VARCHAR(16) NOT NULL DEFAULT 'success',

    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(128)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NULL,

    before_data JSON NULL,
    after_data JSON NULL,

    ip_address VARBINARY(16) NULL,
    user_agent VARCHAR(512) NULL,

    request_id CHAR(26)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT chk_audit_logs_outcome
        CHECK (outcome IN ('success', 'failure')),

    CONSTRAINT fk_audit_logs_actor
        FOREIGN KEY (actor_user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_audit_logs_actor_time (
        actor_user_id,
        created_at
    ),

    INDEX idx_audit_logs_entity_time (
        entity_type,
        entity_id,
        created_at
    ),

    INDEX idx_audit_logs_action_time (
        action,
        created_at
    ),

    INDEX idx_audit_logs_request (
        request_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;