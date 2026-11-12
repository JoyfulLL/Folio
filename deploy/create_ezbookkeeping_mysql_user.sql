-- =============================================================================
-- ezbookkeeping 专用 MySQL 账号初始化脚本
--
-- 目标：账号 ezbookkeeping 仅能操作 ezbookkeeping 库，只能做应用运行所需的
--       常规操作（增删改查 + 自动建表/改表），不具备任何管理权限。
--
-- 执行方式（用 root 或其他有 CREATE USER / GRANT 权限的账号）：
--   mysql -u root -p < create_ezbookkeeping_mysql_user.sql
-- 或在 MySQL 容器内：
--   docker exec -i <mysql容器名> mysql -u root -p<root密码> < create_ezbookkeeping_mysql_user.sql
--
-- 兼容 MySQL 5.7 / 8.0 与 MariaDB 10.x。
-- =============================================================================

-- 【执行前必须修改】把下面两处占位密码换成你自己的强密码（两处要一致）。
-- 不要用弱密码，也不要把真实密码提交到代码仓库。

-- 1) 创建数据库（已存在则跳过，不会覆盖已有数据）
CREATE DATABASE IF NOT EXISTS `ezbookkeeping`
    DEFAULT CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

-- 2) 创建专用账号
--    'localhost' 适用于应用与 MySQL 在同一台机器的情况，最安全。
--    应用跑在容器里时改成容器网段，例如 '172.18.0.%'；
--    应用跑在另一台服务器时改成该服务器的 IP，例如 '192.168.1.20'。
--    尽量避免 '%'（允许任意来源）。若确实需要多个来源，就分别创建多个同名账号。
--
--    MySQL 8.0 默认 caching_sha2_password，本项目 go-sql-driver/mysql v1.10.1 支持，
--    无需改动。若遇到老客户端报认证插件错误，在末尾追加：
--    IDENTIFIED WITH mysql_native_password BY '...'  （即改用下面注释掉的写法）
CREATE USER IF NOT EXISTS 'ezbookkeeping'@'localhost'
    IDENTIFIED BY 'REPLACE_WITH_A_STRONG_PASSWORD';

-- 3) 仅授权 ezbookkeeping 库
--    数据操作 DML：SELECT / INSERT / UPDATE / DELETE
--    结构同步 DDL：CREATE / ALTER / INDEX / REFERENCES
--       -> 项目 conf/ezbookkeeping.ini 中 auto_update_database = true，
--          启动时 xorm.Sync2 会自动建表、加列、加索引，缺了这些权限会启动失败。
--    刻意不给 DROP：MySQL 的库级 DROP 权限同时允许 DROP DATABASE，
--      而本项目代码不执行 DROP TABLE / DROP COLUMN / TRUNCATE，因此无需授予，
--      这样账号就无法删库删表，符合最小权限原则。
--    刻意不给 GRANT OPTION：账号不能把自己或别人的权限再授出去。
--    不授任何全局权限（ON *.*），因此看不到也碰不到其他数据库。
GRANT SELECT, INSERT, UPDATE, DELETE,
      CREATE, ALTER, INDEX, REFERENCES
    ON `ezbookkeeping`.* TO 'ezbookkeeping'@'localhost';

-- 4) 可选补充权限，按需手动放开（默认注释，保持最小权限）
--    CREATE TEMPORARY TABLES / LOCK TABLES  : 手工导数据、mysqldump 备份时可能需要
--    CREATE VIEW / SHOW VIEW                : 自建视图做报表时需要
--    DROP                                  : 仅当未来升级脚本明确要求删表时再开
-- GRANT CREATE TEMPORARY TABLES, LOCK TABLES, CREATE VIEW, SHOW VIEW
--     ON `ezbookkeeping`.* TO 'ezbookkeeping'@'localhost';
-- GRANT DROP ON `ezbookkeeping`.* TO 'ezbookkeeping'@'localhost';

-- MySQL 8.0 的 GRANT 会立即生效；FLUSH PRIVILEGES 为兼容 5.7 / MariaDB 保留。
FLUSH PRIVILEGES;

-- =============================================================================
-- 验证（另起一个会话执行）
-- =============================================================================
-- mysql -u ezbookkeeping -p -h 127.0.0.1 -e "SHOW GRANTS FOR CURRENT_USER();"
-- mysql -u ezbookkeeping -p -h 127.0.0.1 -e "SHOW DATABASES;"  -- 只应看到 information_schema 和 ezbookkeeping
-- mysql -u ezbookkeeping -p -h 127.0.0.1 -e "CREATE DATABASE should_fail;"  -- 应报权限错误，符合预期
--
-- 应用侧对应修改 conf/ezbookkeeping.ini 的 [database] 段：
--   type   = mysql
--   host   = 127.0.0.1:3306
--   name   = ezbookkeeping
--   user   = ezbookkeeping
--   passwd = <与上面一致的强密码>
-- =============================================================================
