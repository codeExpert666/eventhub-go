CREATE TABLE system_bootstrap_record (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    environment VARCHAR(32) NOT NULL,
    note VARCHAR(128) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_system_bootstrap_record_environment
    ON system_bootstrap_record (environment);

INSERT INTO system_bootstrap_record (environment, note)
VALUES ('shared', 'backend foundation initialized');
