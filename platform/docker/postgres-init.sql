CREATE USER media_service WITH PASSWORD 'media_local';
CREATE USER inventory_service WITH PASSWORD 'inventory_local';
CREATE USER payment_service WITH PASSWORD 'payment_local';
CREATE USER order_service WITH PASSWORD 'order_local';

CREATE DATABASE media OWNER media_service;
CREATE DATABASE inventory OWNER inventory_service;
CREATE DATABASE payment OWNER payment_service;
CREATE DATABASE orders OWNER order_service;
