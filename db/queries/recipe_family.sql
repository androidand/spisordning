-- name: CreateRecipeFamily :exec
INSERT INTO recipe_family (id, slug, name, description, default_variant_id, archived, created_at)
VALUES ($1, $2, $3, $4, $5, false, $6);

-- name: GetRecipeFamily :one
SELECT id, slug, name, description, default_variant_id, archived, created_at
FROM recipe_family WHERE id = $1;

-- name: GetRecipeFamilyBySlug :one
SELECT id, slug, name, description, default_variant_id, archived, created_at
FROM recipe_family WHERE slug = $1;

-- name: ListRecipeFamilies :many
SELECT id, slug, name, description, default_variant_id, archived, created_at
FROM recipe_family
ORDER BY slug;

-- name: SetRecipeFamilyDefaultVariant :exec
UPDATE recipe_family SET default_variant_id = $2 WHERE id = $1;

-- name: CreateRecipeVariant :exec
INSERT INTO recipe_variant (id, slug, family_id, title, source_attribution, archived, created_at)
VALUES ($1, $2, $3, $4, $5, false, $6);

-- name: GetRecipeVariant :one
SELECT id, slug, family_id, title, source_attribution, archived, created_at
FROM recipe_variant WHERE id = $1;

-- name: ListRecipeVariants :many
SELECT id, slug, family_id, title, source_attribution, archived, created_at
FROM recipe_variant
WHERE family_id = $1
ORDER BY slug;

-- name: CreateRecipeRevision :one
INSERT INTO recipe_revision (id, variant_id, servings, description, ingredients, steps, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: GetRecipeRevision :one
SELECT id, variant_id, servings, description, ingredients, steps, created_at
FROM recipe_revision WHERE id = $1;

-- name: ListRecipeRevisions :many
SELECT id, variant_id, servings, description, ingredients, steps, created_at
FROM recipe_revision
WHERE variant_id = $1
ORDER BY created_at DESC;

-- name: AddRecipeRevisionParent :exec
INSERT INTO recipe_revision_parent (revision_id, parent_revision_id)
VALUES ($1, $2)
ON CONFLICT (revision_id, parent_revision_id) DO NOTHING;

-- name: ListRecipeRevisionParents :many
SELECT parent_revision_id
FROM recipe_revision_parent
WHERE revision_id = $1
ORDER BY parent_revision_id;
