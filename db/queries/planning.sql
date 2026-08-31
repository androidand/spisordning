-- name: ListEffortProfiles :many
SELECT weekday, kitchen_energy
FROM effort_profile
ORDER BY weekday;
