-- 1. Create comments table
CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    user_id UUID NOT NULL REFERENCES users(id),

    organization_id UUID NOT NULL REFERENCES organizations(id),

    parent_comment_id UUID NULL REFERENCES comments(id) ON DELETE CASCADE,

    content TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);