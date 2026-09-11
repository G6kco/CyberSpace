CREATE TABLE ip_allocations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    network_address_id BIGINT UNSIGNED NOT NULL,
    lab_instance_service_id BIGINT UNSIGNED NOT NULL,

    allocated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    released_at DATETIME(6) NULL,

    active_network_address_id BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN released_at IS NULL THEN network_address_id
                ELSE NULL
            END
        ) STORED,

    active_service_id BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN released_at IS NULL THEN lab_instance_service_id
                ELSE NULL
            END
        ) STORED,

    PRIMARY KEY (id),

    CONSTRAINT uq_ip_allocations_active_address
        UNIQUE (active_network_address_id),

    CONSTRAINT uq_ip_allocations_active_service
        UNIQUE (active_service_id),

    CONSTRAINT chk_ip_allocations_release_time
        CHECK (
            released_at IS NULL
            OR released_at >= allocated_at
        ),

    CONSTRAINT fk_ip_allocations_address
        FOREIGN KEY (network_address_id)
        REFERENCES network_addresses (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_ip_allocations_service
        FOREIGN KEY (lab_instance_service_id)
        REFERENCES lab_instance_services (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_ip_allocations_service_history (
        lab_instance_service_id,
        allocated_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;