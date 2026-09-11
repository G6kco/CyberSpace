CREATE TABLE student_profiles (
    user_id BIGINT UNSIGNED NOT NULL,
    register_number VARCHAR(64)
    CHARACTER SET ascii
    COLLATE ascii_bin
    NOT NULL,
    department VARCHAR(120) NOT NULL,
    programme VARCHAR(120) NOT NULL,
    graduation_year SMALLINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
    DEFAULT CURRENT_TIMESTAMP(6)
    ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (user_id),

    CONSTRAINT uq_student_profiles_register_number
        UNIQUE (register_number),

    CONSTRAINT chk_student_profiles_graduation_year
        CHECK (graduation_year BETWEEN 2000 AND 2200),

    CONSTRAINT fk_student_profiles_user
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_student_profiles_department_graduation_year (
    department,
    graduation_year
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;