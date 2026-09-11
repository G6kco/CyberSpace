CREATE TABLE assessment_access_grants (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    user_id BIGINT UNSIGNED NOT NULL,
    assessment_version_id BIGINT UNSIGNED NOT NULL,

    source VARCHAR(32) NOT NULL,
    external_reference VARCHAR(255) NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'pending',

    eligible_from DATETIME(6) NOT NULL,
    eligible_until DATETIME(6) NOT NULL,

    approved_by BIGINT UNSIGNED NULL,
    approved_at DATETIME(6) NULL,

    consumed_at DATETIME(6) NULL,

    revoked_by BIGINT UNSIGNED NULL,
    revoked_at DATETIME(6) NULL,
    revocation_reason VARCHAR(500) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_access_grants_external_reference
        UNIQUE (source, external_reference),

    CONSTRAINT chk_access_grants_source
        CHECK (
            source IN (
                'external_booking',
                'manual_admin'
            )
        ),

    CONSTRAINT chk_access_grants_status
        CHECK (
            status IN (
                'pending',
                'approved',
                'consumed',
                'revoked',
                'expired'
            )
        ),

    CONSTRAINT chk_access_grants_window
        CHECK (eligible_until > eligible_from),

    CONSTRAINT chk_access_grants_external_source
        CHECK (
            (
                source = 'external_booking'
                AND external_reference IS NOT NULL
            )
            OR
            (
                source = 'manual_admin'
                AND external_reference IS NULL
            )
        ),

    CONSTRAINT chk_access_grants_approval_pair
        CHECK (
            (
                approved_by IS NULL
                AND approved_at IS NULL
            )
            OR
            (
                approved_by IS NOT NULL
                AND approved_at IS NOT NULL
            )
        ),

    CONSTRAINT chk_access_grants_approval_status
        CHECK (
            status NOT IN ('approved', 'consumed')
            OR
            (
                approved_by IS NOT NULL
                AND approved_at IS NOT NULL
            )
        ),

    CONSTRAINT chk_access_grants_consumption
        CHECK (
            (
                status = 'consumed'
                AND consumed_at IS NOT NULL
            )
            OR
            (
                status <> 'consumed'
                AND consumed_at IS NULL
            )
        ),

    CONSTRAINT chk_access_grants_revocation
        CHECK (
            (
                status = 'revoked'
                AND revoked_by IS NOT NULL
                AND revoked_at IS NOT NULL
                AND revocation_reason IS NOT NULL
                AND CHAR_LENGTH(TRIM(revocation_reason)) > 0
            )
            OR
            (
                status <> 'revoked'
                AND revoked_by IS NULL
                AND revoked_at IS NULL
                AND revocation_reason IS NULL
            )
        ),

    CONSTRAINT fk_access_grants_user
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_access_grants_assessment_version
        FOREIGN KEY (assessment_version_id)
        REFERENCES assessment_versions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_access_grants_approved_by
        FOREIGN KEY (approved_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_access_grants_revoked_by
        FOREIGN KEY (revoked_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_access_grants_user_status (
        user_id,
        status,
        eligible_from,
        eligible_until
    ),

    INDEX idx_access_grants_version_status (
        assessment_version_id,
        status
    ),

    INDEX idx_access_grants_expiration (
        status,
        eligible_until
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;