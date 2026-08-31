-- name: CreatePerson :exec
INSERT INTO person (id, name, weight)
VALUES ($1, $2, $3);

-- name: UpdatePerson :exec
UPDATE person SET name = $2, weight = $3 WHERE id = $1;

-- name: GetPerson :one
SELECT id, name, weight FROM person WHERE id = $1;

-- name: ListPeople :many
SELECT id, name, weight FROM person ORDER BY name;

-- name: UpsertPreference :exec
INSERT INTO person_preference (person_id, tag, sentiment, confidence)
VALUES ($1, $2, $3, $4)
ON CONFLICT (person_id, tag) DO UPDATE
SET sentiment = EXCLUDED.sentiment,
    confidence = EXCLUDED.confidence;

-- name: ListPreferences :many
SELECT person_id, tag, sentiment, confidence
FROM person_preference
WHERE person_id = $1
ORDER BY tag;

-- name: RecordObservation :exec
INSERT INTO preference_observation (person_id, tag, sentiment, observed_at)
VALUES ($1, $2, $3, $4);
