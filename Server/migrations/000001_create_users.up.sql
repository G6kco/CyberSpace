CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    email VARCHAR(254) NOT NULL,
    display_name VARCHAR(150) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'student',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    google_subject VARCHAR(255)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NULL,
    last_login_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_users_public_id
        UNIQUE (public_id),

    CONSTRAINT uq_users_email
        UNIQUE (email),

    CONSTRAINT uq_users_google_subject
        UNIQUE (google_subject),

    CONSTRAINT chk_users_role
        CHECK (role IN ('student', 'admin')),

    CONSTRAINT chk_users_status
        CHECK (status IN ('active', 'suspended', 'archived')),

    INDEX idx_users_role_status (role, status)
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;