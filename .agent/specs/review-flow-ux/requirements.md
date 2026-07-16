# Requirements Document

## Introduction

This feature makes text entry and test-suite review clearer without enlarging the primary setup canvas. It exposes weighted scoring criteria, lets users request an additional test, reduces footer clutter, and uses `Enter` consistently for visible actions.

## Glossary

- **mdbench**: The evaluation application.
- **Test suite**: A saved, immutable revision of test cases and its scoring contract.
- **Weighted scoring criterion**: A scored quality category with a weight, guidance, and 0/5/10 rating anchors.
- **Requested test**: One additional generated test case based on a user's plain-language request.

## Requirements

### Requirement 1: Reusable text entry

**User Story:** As a user, I want multiline input to have a visible completion action so that it works in any supported terminal.

#### Acceptance Criteria

1. WHEN multiline input opens, THE mdbench SHALL show the editor and a visible action below it.
2. WHEN the editor is focused, THE mdbench SHALL use `Enter` for a new line and `Tab` to move focus to the action.
3. WHEN the action is focused and the user presses `Enter`, THE mdbench SHALL accept valid text and continue.
4. WHEN accepted text is empty, THE mdbench SHALL remain on the editor and show an inline correction.
5. THE mdbench SHALL NOT require `Command+Enter` or `Ctrl+S` to complete multiline input.
6. THE mdbench SHALL reuse the same text-entry interaction for pasted input, test requests, case prompts, and rubric text.

### Requirement 2: Upfront weighted scoring criteria

**User Story:** As a skill author, I want to see how results will be scored while reviewing tests so that the evaluation contract is clear.

#### Acceptance Criteria

1. WHEN test-suite review opens, THE mdbench SHALL show the test-case list and a visible summary labeled `Weighted scoring criteria` on the same screen.
2. WHEN a test case is generated, THE mdbench SHALL assign the subset of enabled criteria relevant to that case.
3. WHEN the user opens a test case, THE mdbench SHALL show the criteria assigned to that case.
4. WHEN the user reviews weighted scoring criteria, THE mdbench SHALL allow each approved criterion to be enabled or disabled and its positive weight to be changed.
5. WHEN a criterion is disabled and later re-enabled, THE mdbench SHALL preserve its generated case assignments.
6. IF disabling a criterion would leave an enabled case with no enabled criterion, THEN THE mdbench SHALL block the change and explain why.
7. THE mdbench SHALL require at least one enabled weighted scoring criterion.
8. THE mdbench SHALL NOT expose arbitrary custom criterion creation in this MVP feature.

### Requirement 3: User-requested tests

**User Story:** As a skill author, I want to request a test the generator missed so that the saved suite covers my specific concern.

#### Acceptance Criteria

1. WHEN the test-case list is shown, THE mdbench SHALL include an `Add a test request` action after the generated cases.
2. WHEN the user activates that action with `Enter`, THE mdbench SHALL open reusable multiline text entry for one plain-language request.
3. WHEN the user accepts a valid request, THE mdbench SHALL show a bounded loading indicator while generating one additional case.
4. WHEN generation succeeds, THE mdbench SHALL validate the case, append it to the current draft, return to the complete test-case list, and highlight the added case.
5. WHEN generation fails, THE mdbench SHALL return to the request editor with the request preserved and an actionable error.
6. WHEN the user wants more requested tests, THE mdbench SHALL allow the action to be repeated before the suite is saved.

### Requirement 4: Save and continue conventions

**User Story:** As a user, I want familiar action wording and keys so that I do not have to learn screen-specific conventions.

#### Acceptance Criteria

1. THE mdbench SHALL use `Save test suite & continue` instead of user-facing `freeze` wording.
2. WHEN a visible continue, confirm, save, or select action is focused, THE mdbench SHALL activate it with `Enter`.
3. WHEN the user saves a new or edited test suite, THE mdbench SHALL create the immutable revision internally and continue directly to evaluation configuration.
4. WHEN an unchanged saved suite is reused, THE mdbench SHALL continue without creating another revision.

### Requirement 5: Focused help and home descriptions

**User Story:** As a new user, I want short explanations and uncluttered controls so that I can understand the flow without reading a manual.

#### Acceptance Criteria

1. WHEN the home screen is shown, THE mdbench SHALL render one short muted description beside each primary option.
2. WHEN a screen footer is shown, THE mdbench SHALL keep only the primary navigation and action controls visible.
3. WHEN the user presses `?`, THE mdbench SHALL show all secondary controls for the current screen in a dismissible help view.
4. WHILE this feature is active, THE mdbench SHALL keep the approved primary setup canvas size and SHALL NOT add a screen-specific size override.
