-- 建筑施工安全管理平台 (safety-platform) 初始化脚本：首次启动容器时自动执行
SET NAMES utf8mb4;
CREATE DATABASE IF NOT EXISTS safety_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE safety_db;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  phone VARCHAR(20) NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  name VARCHAR(50) NOT NULL DEFAULT '',
  avatar VARCHAR(255) NOT NULL DEFAULT '',
  role VARCHAR(30) NOT NULL DEFAULT 'worker',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_phone (phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS safety_incidents (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  title VARCHAR(200) NOT NULL,
  description TEXT,
  occurred_at DATETIME NOT NULL,
  site_id VARCHAR(50) NOT NULL DEFAULT '',
  area VARCHAR(100) NOT NULL DEFAULT '',
  severity_level VARCHAR(30) NOT NULL DEFAULT 'minor',
  category VARCHAR(50) NOT NULL DEFAULT '其他',
  involved_user_ids JSON,
  photo_urls JSON,
  status VARCHAR(30) NOT NULL DEFAULT 'reported',
  rectification_measures TEXT,
  rectification_deadline DATETIME,
  reporter_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_incidents_status (status),
  KEY idx_incidents_severity (severity_level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS safety_inspections (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(200) NOT NULL,
  inspection_type VARCHAR(30) NOT NULL DEFAULT 'routine',
  area VARCHAR(100) NOT NULL DEFAULT '',
  inspection_date DATETIME NOT NULL,
  inspector_id BIGINT UNSIGNED NOT NULL,
  total_score INT NOT NULL DEFAULT 0,
  status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
  issue_count INT NOT NULL DEFAULT 0,
  passed_count INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_inspections_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inspection_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  inspection_id BIGINT UNSIGNED NOT NULL,
  item_name VARCHAR(200) NOT NULL,
  passed TINYINT(1) NOT NULL DEFAULT 0,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  photo_url VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_items_inspection (inspection_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 整改任务：每个不合格检查项至多一条（uk_rect_tasks_item），退回/重提在同一条上流转
CREATE TABLE IF NOT EXISTS rectification_tasks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  inspection_id BIGINT UNSIGNED NOT NULL,
  item_id BIGINT UNSIGNED NOT NULL,
  item_name VARCHAR(200) NOT NULL DEFAULT '',
  assignee_id BIGINT UNSIGNED NOT NULL,
  deadline DATETIME NOT NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'pending',
  rectification_note VARCHAR(500) NOT NULL DEFAULT '',
  rectification_photo VARCHAR(255) NOT NULL DEFAULT '',
  submitted_at DATETIME NULL,
  reviewer_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reviewed_at DATETIME NULL,
  review_note VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_rect_tasks_item (item_id),
  KEY idx_rect_tasks_inspection (inspection_id),
  KEY idx_rect_tasks_assignee (assignee_id),
  KEY idx_rect_tasks_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 整改流转记录：登记/提交/退回/通过均追加一条，原记录保留不可改
CREATE TABLE IF NOT EXISTS rectification_histories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  task_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(30) NOT NULL,
  note VARCHAR(500) NOT NULL DEFAULT '',
  photo_url VARCHAR(255) NOT NULL DEFAULT '',
  operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_rect_hist_task (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS safety_trainings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  topic VARCHAR(200) NOT NULL,
  training_type VARCHAR(30) NOT NULL DEFAULT 'regular',
  training_date DATETIME NOT NULL,
  duration_hours INT NOT NULL DEFAULT 1,
  trainer VARCHAR(50) NOT NULL DEFAULT '',
  location VARCHAR(255) NOT NULL DEFAULT '',
  content_summary TEXT,
  participant_ids JSON,
  assessment_method VARCHAR(30) NOT NULL DEFAULT '笔试',
  pass_rate DECIMAL(5,2) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS worker_certifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  cert_type VARCHAR(50) NOT NULL DEFAULT '',
  cert_no VARCHAR(50) NOT NULL DEFAULT '',
  issue_org VARCHAR(100) NOT NULL DEFAULT '',
  issue_date DATE,
  valid_until DATE,
  cert_photo_url VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(30) NOT NULL DEFAULT 'pending',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_certs_user (user_id),
  KEY idx_certs_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  operator_name VARCHAR(50) NOT NULL DEFAULT '',
  action VARCHAR(50) NOT NULL,
  entity_type VARCHAR(50) NOT NULL,
  entity_id VARCHAR(50) NOT NULL DEFAULT '',
  detail TEXT,
  ip VARCHAR(50) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 预置种子数据（密码：admin/Admin@123，其余/User@123）
INSERT INTO users (id, phone, password_hash, name, avatar, role, created_at) VALUES
(1, '13800000001', '$2a$10$bFfMuQAuKWflKxpuDYdFpeGJPVgD83q/.278LHYLL5S0DDmEfChX2', '系统管理员', '', 'admin', NOW(3)),
(2, '13800000002', '$2a$10$TMTpnDbEwRbtcbF9VJxAxe5IswQjmo7pboKI9zVtU.BYnhzdJpX9a', '王安全', '', 'safety_manager', NOW(3)),
(3, '13800000003', '$2a$10$TMTpnDbEwRbtcbF9VJxAxe5IswQjmo7pboKI9zVtU.BYnhzdJpX9a', '李监理', '', 'inspector', NOW(3)),
(4, '13800000004', '$2a$10$TMTpnDbEwRbtcbF9VJxAxe5IswQjmo7pboKI9zVtU.BYnhzdJpX9a', '赵工', '', 'worker', NOW(3));

INSERT INTO safety_incidents (id, title, description, occurred_at, site_id, area, severity_level, category, involved_user_ids, photo_urls, status, rectification_measures, rectification_deadline, reporter_id, created_at) VALUES
(1, '脚手架扣件松动', '三层东侧脚手架扣件松动，存在坠落风险。', DATE_SUB(NOW(), INTERVAL 1 DAY), 'SITE-A', '三层东侧', 'major', '坠落', '["4"]', '[]', 'investigating', '重新紧固并加装防坠网', DATE_ADD(NOW(), INTERVAL 3 DAY), 3, NOW(3)),
(2, '临时用电电缆破损', '二级配电箱电缆绝缘层破损。', DATE_SUB(NOW(), INTERVAL 2 DAY), 'SITE-A', '加工区', 'moderate', '触电', '["4"]', '[]', 'resolved', '更换破损电缆并加套管', DATE_SUB(NOW(), INTERVAL 1 DAY), 3, NOW(3)),
(3, '高处坠物未遂', '塔吊吊运时构件滑落未造成伤害。', DATE_SUB(NOW(), INTERVAL 5 DAY), 'SITE-A', '吊装区', 'minor', '物体打击', '["4"]', '[]', 'closed', '加强吊装指挥与警戒', DATE_SUB(NOW(), INTERVAL 2 DAY), 2, NOW(3));

INSERT INTO safety_inspections (id, name, inspection_type, area, inspection_date, inspector_id, total_score, status, issue_count, passed_count, created_at) VALUES
(1, '8月例行安全检查', 'routine', '全工地', DATE_SUB(NOW(), INTERVAL 1 DAY), 3, 60, 'failed', 2, 3, NOW(3)),
(2, '高处作业专项检查', 'special', '三层作业面', DATE_ADD(NOW(), INTERVAL 1 DAY), 3, 0, 'scheduled', 0, 0, NOW(3));

INSERT INTO inspection_items (id, inspection_id, item_name, passed, remark, photo_url) VALUES
(1, 1, '安全帽佩戴', 1, '', ''),
(2, 1, '临边防护栏杆', 0, '东侧栏杆缺失', ''),
(3, 1, '消防器材齐全', 1, '', ''),
(4, 1, '配电箱接地保护', 0, '加工区二级箱未做重复接地', ''),
(5, 1, '安全通道畅通', 1, '', '');

INSERT INTO rectification_tasks (id, inspection_id, item_id, item_name, assignee_id, deadline, status, rectification_note, rectification_photo, submitted_at, reviewer_id, reviewed_at, review_note, created_at, updated_at) VALUES
(1, 1, 2, '临边防护栏杆', 4, DATE_ADD(NOW(), INTERVAL 2 DAY), 'submitted', '已补装东侧临边防护栏杆并挂设密目安全网，请复查。', '', DATE_SUB(NOW(), INTERVAL 2 HOUR), 0, NULL, '', NOW(3), DATE_SUB(NOW(), INTERVAL 2 HOUR)),
(2, 1, 4, '配电箱接地保护', 4, DATE_SUB(NOW(), INTERVAL 1 DAY), 'pending', '', '', NULL, 0, NULL, '', NOW(3), NOW(3));

INSERT INTO rectification_histories (id, task_id, action, note, photo_url, operator_id, created_at) VALUES
(1, 1, 'register', CONCAT('8月例行安全检查 登记整改任务，期限 ', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 2 DAY), '%Y-%m-%d')), '', 3, NOW(3)),
(2, 1, 'submit', '已补装东侧临边防护栏杆并挂设密目安全网，请复查。', '', 4, DATE_SUB(NOW(), INTERVAL 2 HOUR)),
(3, 2, 'register', CONCAT('8月例行安全检查 登记整改任务，期限 ', DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 1 DAY), '%Y-%m-%d')), '', 3, NOW(3));

INSERT INTO safety_trainings (id, topic, training_type, training_date, duration_hours, trainer, location, content_summary, participant_ids, assessment_method, pass_rate, created_at) VALUES
(1, '新员工入场安全培训', 'induction', DATE_SUB(NOW(), INTERVAL 3 DAY), 4, '王安全', '培训室A', '入场安全须知与应急疏散。', '["4"]', '笔试', 95.00, NOW(3)),
(2, '高空作业专项培训', 'special', DATE_ADD(NOW(), INTERVAL 2 DAY), 2, '王安全', '培训室B', '高空作业规范与防护用品使用。', '[]', '实操', 0.00, NOW(3));

INSERT INTO worker_certifications (id, user_id, cert_type, cert_no, issue_org, issue_date, valid_until, cert_photo_url, status, created_at) VALUES
(1, 4, '特种作业证', 'TZ20260001', '市应急管理局', DATE_SUB(NOW(), INTERVAL 100 DAY), DATE_ADD(NOW(), INTERVAL 200 DAY), '', 'approved', NOW(3)),
(2, 3, '安全员证', 'AQ20260002', '市住建局', DATE_SUB(NOW(), INTERVAL 50 DAY), DATE_ADD(NOW(), INTERVAL 300 DAY), '', 'pending', NOW(3)),
(3, 2, '电工证', 'DG20260003', '市住建局', DATE_SUB(NOW(), INTERVAL 400 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), '', 'approved', NOW(3));

INSERT INTO audit_logs (id, operator_id, operator_name, action, entity_type, entity_id, detail, ip, created_at) VALUES
(1, 1, 'admin', 'seed', 'system', '', 'init', '127.0.0.1', NOW(3));
