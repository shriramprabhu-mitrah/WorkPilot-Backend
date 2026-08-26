-- Migration to remove redundant 'comments:comment' permission and migrate roles using it to 'comments:add'

-- 1. Insert 'comments:add' permission for roles that currently have 'comments:comment' but lack 'comments:add'
INSERT INTO role_permissions (role_id, permission_id)
SELECT rp.role_id, (SELECT id FROM permissions WHERE resource = 'comments' AND action = 'add')
FROM role_permissions rp
JOIN permissions p ON rp.permission_id = p.id
WHERE p.resource = 'comments' AND p.action = 'comment'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp2
      JOIN permissions p2 ON rp2.permission_id = p2.id
      WHERE rp2.role_id = rp.role_id AND p2.resource = 'comments' AND p2.action = 'add'
  )
ON CONFLICT DO NOTHING;

-- 2. Delete 'comments:comment' records from role_permissions
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE resource = 'comments' AND action = 'comment');

-- 3. Delete 'comments:comment' from permissions table
DELETE FROM permissions
WHERE resource = 'comments' AND action = 'comment';
