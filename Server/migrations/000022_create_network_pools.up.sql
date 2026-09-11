CREATE TABLE network_pools (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    docker_host_id BIGINT UNSIGNED NOT NULL,

    name VARCHAR(160) NOT NULL,
    cidr VARCHAR(43)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NOT NULL,

    gateway VARBINARY(16) NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_network_pools_host_name
        UNIQUE (docker_host_id, name),

    CONSTRAINT uq_network_pools_host_cidr
        UNIQUE (docker_host_id, cidr),

    CONSTRAINT chk_network_pools_status
        CHECK (status IN ('active', 'disabled')),

    CONSTRAINT fk_network_pools_docker_host
        FOREIGN KEY (docker_host_id)
        REFERENCES docker_hosts (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_network_pools_host_status (
        docker_host_id,
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;