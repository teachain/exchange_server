CREATE TABLE IF NOT EXISTS operlog (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    type INT NOT NULL,
    data TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS slice_balance (
    user_id BIGINT NOT NULL,
    asset VARCHAR(32) NOT NULL,
    balance DECIMAL(20,8) NOT NULL DEFAULT 0,
    frozen DECIMAL(20,8) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, asset),
    INDEX idx_user_id (user_id)
);

CREATE TABLE IF NOT EXISTS slice_order (
    order_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    market VARCHAR(32) NOT NULL,
    side INT NOT NULL,
    price DECIMAL(20,8) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    deal DECIMAL(20,8) NOT NULL DEFAULT 0,
    fee DECIMAL(20,8) NOT NULL DEFAULT 0,
    status INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_market_status (market, status)
);

CREATE TABLE IF NOT EXISTS slice_history (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    last_id BIGINT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);