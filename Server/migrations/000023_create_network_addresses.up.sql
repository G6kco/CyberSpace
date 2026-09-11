CREATE TABLE network_addresses (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    network_pool_id BIGINT UNSIGNED NOT NULL,
    ip_address VARBINARY(16) NOT NULL,

    is_reserved BOOLEAN NOT NULL DEFAULT FALSE,
    reservation_reason VARCHAR(255) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_network_addresses_pool_ip
        UNIQUE (network_pool_id, ip_address),

    CONSTRAINT chk_network_addresses_reservation
        CHECK (
            (
                is_reserved = FALSE
                AND reservation_reason IS NULL
            )
            OR
            (
                is_reserved = TRUE
                AND reservation_reason IS NOT NULL
                AND CHAR_LENGTH(TRIM(reservation_reason)) > 0
            )
        ),

    CONSTRAINT fk_network_addresses_pool
        FOREIGN KEY (network_pool_id)
        REFERENCES network_pools (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_network_addresses_pool_reserved (
        network_pool_id,
        is_reserved
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;