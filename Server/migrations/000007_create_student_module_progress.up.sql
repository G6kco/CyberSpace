CREATE TABLE student_module_progress (
    user_id BIGINT UNSIGNED NOT NULL,
    module_id BIGINT UNSIGNED NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'in_progress',
    progress_percent DECIMAL(5,2) UNSIGNED NOT NULL DEFAULT 0.00,

    started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    completed_at DATETIME(6) NULL,
    last_accessed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (user_id, module_id),

    CONSTRAINT chk_student_module_progress_status
        CHECK (status IN ('in_progress', 'completed')),

    CONSTRAINT chk_student_module_progress_percentage
        CHECK (progress_percent BETWEEN 0.00 AND 100.00),

    CONSTRAINT chk_student_module_progress_completion
        CHECK (
            (
                status = 'in_progress'
                AND completed_at IS NULL
                AND progress_percent < 100.00
            )
            OR
            (
                status = 'completed'
                AND completed_at IS NOT NULL
                AND progress_percent = 100.00
            )
        ),

    CONSTRAINT fk_student_module_progress_user
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_student_module_progress_module
        FOREIGN KEY (module_id)
        REFERENCES learning_modules (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_student_module_progress_user_status (
        user_id,
        status
    ),

    INDEX idx_student_module_progress_module_status (
        module_id,
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;