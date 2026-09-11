CREATE TABLE docker_hosts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    name VARCHAR(160) NOT NULL,
    endpoint_reference VARCHAR(255) NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'offline',
    max_instances INT UNSIGNED NOT NULL,

    last_heartbeat_at DATETIME(6) NULL,
    metadata_json JSON NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_docker_hosts_name
        UNIQUE (name),

    CONSTRAINT uq_docker_hosts_endpoint_reference
        UNIQUE (endpoint_reference),

    CONSTRAINT chk_docker_hosts_status
        CHECK (
            status IN (
                'online',
                'offline',
                'maintenance',
                'disabled'
            )
        ),

    CONSTRAINT chk_docker_hosts_capacity
        CHECK (max_instances > 0),

    INDEX idx_docker_hosts_status_heartbeat (
        status,
        last_heartbeat_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;