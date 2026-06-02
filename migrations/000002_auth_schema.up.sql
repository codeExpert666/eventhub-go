CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(32) NOT NULL,
    email VARCHAR(128) NOT NULL,
    password_hash VARCHAR(100) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_users_username UNIQUE (username),
    CONSTRAINT uk_users_email UNIQUE (email)
);

CREATE TABLE roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_roles_code UNIQUE (code)
);

CREATE TABLE user_roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_user_roles_user_role UNIQUE (user_id, role_id),
    CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES roles (id)
);

CREATE INDEX idx_user_roles_role_id
    ON user_roles (role_id);

INSERT INTO roles (code, name, description)
VALUES
    ('USER', '普通用户', '可以创建订单、查看自己的订单并完成模拟支付'),
    ('ADMIN', '管理员', '可以管理活动、场次、票种并查看平台操作日志');

INSERT INTO users (username, email, password_hash, status)
VALUES (
    'admin',
    'admin@eventhub.local',
    '$2y$10$5PN1JiDDf45mbKMEivrs7ucz63JaKGqD8zjS0zoOoqdhoih4byXFy',
    'ENABLED'
);

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('USER', 'ADMIN')
WHERE u.username = 'admin';

CREATE TABLE auth_sessions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL,
    refresh_token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    last_refreshed_at TIMESTAMP NULL,
    last_seen_at TIMESTAMP NULL,
    revoked_at TIMESTAMP NULL,
    revoke_reason VARCHAR(64) NULL,
    client_ip_hash VARCHAR(128) NULL,
    user_agent_hash VARCHAR(128) NULL,
    user_agent_summary VARCHAR(255) NULL,
    version INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_auth_sessions_session_id UNIQUE (session_id),
    CONSTRAINT uk_auth_sessions_refresh_token_hash UNIQUE (refresh_token_hash),
    CONSTRAINT fk_auth_sessions_user FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX idx_auth_sessions_user_id
    ON auth_sessions (user_id);

CREATE INDEX idx_auth_sessions_status
    ON auth_sessions (status);

CREATE INDEX idx_auth_sessions_refresh_expires_at
    ON auth_sessions (refresh_expires_at);
