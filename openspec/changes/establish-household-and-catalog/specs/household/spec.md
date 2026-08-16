# household (delta)

## ADDED Requirements

### Requirement: Login identity is distinct from household Person

The system SHALL model `Account` (login identity) and `Person` (household food-domain member)
as separate entities connected by an optional reference. A `Person` SHALL be able to exist
with no linked `Account`.

#### Scenario: A child has no login but is a full household member

- **WHEN** a household adds a person who has no login credentials
- **THEN** a `Person` is created with preferences, restrictions, and meal history available
- **AND** no `Account` is required to exist for that `Person`

#### Scenario: Removing an Account does not remove the Person

- **WHEN** an `Account` linked to a `Person` is deleted
- **THEN** the `Person` and their preference/restriction/meal history are unaffected

### Requirement: Household membership has history

Ending a person's membership in a household SHALL close the membership record, not delete the
`Person` or any of their preference, restriction, or meal history.

#### Scenario: A former member's meal history remains intact

- **WHEN** a `HouseholdMembership` is ended for a person who has past meal reactions
- **THEN** the `Person` record and their past `meal_reaction` rows remain queryable
- **AND** the household membership is marked ended, not deleted

### Requirement: Preferences and restrictions are never merged

The system SHALL keep `PersonPreference` (LIKES/DISLIKES) and `PersonRestriction`
(ALLERGY/HARD_RESTRICTION) as separate models. The system SHALL NOT use a restriction as
input to a preference sentiment or confidence computation, and SHALL NOT use a preference to
imply, weaken, or override a restriction.

#### Scenario: An allergy is never scored as a preference

- **WHEN** a person's allergy to peanuts is recorded
- **THEN** no `person_preference` row is created or modified for "peanuts" as a result
- **AND** the allergy has no `sentiment`/`confidence` score

#### Scenario: A dislike cannot become a restriction

- **WHEN** a person repeatedly reacts negatively to an ingredient (lowering preference
  confidence toward strong dislike)
- **THEN** no `person_restriction` row is created or modified as a result
- **AND** only an explicit restriction command can create a `person_restriction`

### Requirement: Restriction changes are explicit and attributed

A `PersonRestriction` SHALL be created, changed, or cleared only through an explicit command
that records who performed the change and when. It SHALL NOT be derived automatically from
`preference_observation`, meal reactions, or any scoring/inference path.

#### Scenario: Setting an allergy requires an explicit actor

- **WHEN** a household member records that a person is allergic to shellfish
- **THEN** the resulting `person_restriction` row records the recording actor and timestamp
- **AND** no automated process could have produced this row without that explicit call
