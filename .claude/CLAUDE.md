# Claude Development Guidelines

## Test-Driven Development (TDD)

**ALWAYS follow Test-Driven Development practices:**

### TDD Cycle (Red-Green-Refactor)

1. **RED** - Write a failing test first
   - Write the test before implementing the feature
   - The test should fail initially (red)
   - This ensures the test actually tests something

2. **GREEN** - Write minimal code to make it pass
   - Implement just enough to make the test pass
   - Don't add extra features "just in case"
   - Focus on making the test green

3. **REFACTOR** - Clean up the code
   - Improve code quality while keeping tests green
   - Remove duplication
   - Improve naming and structure

### TDD Workflow Example

```python
# Step 1: RED - Write failing test
def test_ball_detector_finds_ball():
    detector = BallDetector(config)
    frame = load_test_frame('ball_visible.jpg')

    result = detector.detect(frame)

    assert result is not None
    assert result.x > 0
    assert result.confidence > 0.5

# Step 2: GREEN - Implement minimal code
class BallDetector:
    def detect(self, frame):
        # Minimal implementation to pass test
        return BallDetection(x=100, y=100, confidence=0.8)

# Step 3: REFACTOR - Improve implementation
class BallDetector:
    def detect(self, frame):
        # Now add real detection logic
        hsv = cv2.cvtColor(frame, cv2.COLOR_BGR2HSV)
        mask = cv2.inRange(hsv, self.lower, self.upper)
        contours = cv2.findContours(mask, ...)
        # ... proper implementation
```

### When to Write Tests

**ALWAYS write tests for:**
- ✅ New features (test first, then implement)
- ✅ Bug fixes (write failing test that reproduces bug, then fix)
- ✅ Refactoring (tests ensure behavior doesn't change)
- ✅ Public APIs and interfaces
- ✅ Business logic and algorithms
- ✅ Edge cases and error conditions

**Optional for:**
- ⚠️  Simple getters/setters (unless they have logic)
- ⚠️  Pure UI code (consider integration tests instead)
- ⚠️  Third-party library wrappers (test your usage, not the library)

## Testing Anti-Patterns to AVOID

### ❌ 1. Testing Implementation Details

**BAD:**
```python
def test_ball_detector_uses_hsv():
    detector = BallDetector()
    # Don't test HOW it works internally
    assert detector._color_space == 'HSV'  # WRONG!
```

**GOOD:**
```python
def test_ball_detector_finds_white_ball():
    detector = BallDetector()
    frame = load_frame_with_white_ball()
    # Test WHAT it does, not HOW
    ball = detector.detect(frame)
    assert ball is not None
    assert ball.confidence > 0.7
```

**Why:** Implementation details change. Test behavior, not internals.

---

### ❌ 2. Fragile Tests (Breaking on Unrelated Changes)

**BAD:**
```python
def test_export_json():
    result = export_to_json(phases)
    # Don't assert exact string matches
    assert result == '{"phases":[{"id":0,"phase":"yellow"}]}'  # WRONG!
```

**GOOD:**
```python
def test_export_json():
    result = export_to_json(phases)
    data = json.loads(result)
    assert 'phases' in data
    assert len(data['phases']) == 1
    assert data['phases'][0]['phase'] == 'yellow'
```

**Why:** Exact string matching breaks on formatting changes.

---

### ❌ 3. Testing Multiple Things in One Test

**BAD:**
```python
def test_everything():
    # Don't test multiple unrelated things
    assert ball_detector.detect(frame1) is not None
    assert phase_classifier.classify(zone) == GamePhase.YELLOW
    assert smoother.smooth(events) == expected
    # WRONG! Too much in one test
```

**GOOD:**
```python
def test_ball_detector_finds_ball():
    assert ball_detector.detect(frame) is not None

def test_phase_classifier_classifies_yellow():
    assert phase_classifier.classify('yellow_five_bar') == GamePhase.YELLOW

def test_smoother_filters_short_phases():
    assert len(smoother.smooth(events)) < len(events)
```

**Why:** When it fails, you don't know what actually broke.

---

### ❌ 4. Tests That Depend on Each Other

**BAD:**
```python
# Global state shared between tests
detector = BallDetector()

def test_first():
    detector.calibrate(frame)  # Modifies detector

def test_second():
    # Depends on test_first running first!
    result = detector.detect(frame)  # WRONG!
```

**GOOD:**
```python
def test_detector_after_calibration():
    detector = BallDetector()  # Fresh instance
    detector.calibrate(frame)
    result = detector.detect(frame)
    assert result is not None

def test_detector_without_calibration():
    detector = BallDetector()  # Fresh instance
    result = detector.detect(frame)
    # Test handles uncalibrated case
```

**Why:** Tests should be independent and run in any order.

---

### ❌ 5. Testing Too Much at Once (No Unit Tests)

**BAD:**
```python
def test_entire_pipeline():
    # Don't test everything end-to-end for every case
    video = VideoProcessor('test.mp4')
    detector = BallDetector()
    classifier = PhaseClassifier()
    smoother = TemporalSmoother()
    # ... 50 lines of setup
    result = full_analysis(video)
    assert result == expected  # WRONG! Too high level
```

**GOOD:**
```python
# Unit test each component
def test_ball_detector():
    detector = BallDetector(config)
    ball = detector.detect(frame)
    assert ball.x == 100

def test_phase_classifier():
    classifier = PhaseClassifier(config)
    phase = classifier.classify_frame(0, 0.0, 'yellow_five_bar', None)
    assert phase == GamePhase.YELLOW_FIVE_BAR

# PLUS one integration test
def test_full_pipeline_integration():
    # Test components working together
    result = analyze_video('test.mp4')
    assert len(result) > 0
```

**Why:** Unit tests are fast and pinpoint failures. Integration tests ensure pieces work together.

---

### ❌ 6. Mocking Everything

**BAD:**
```python
def test_with_too_many_mocks():
    mock_video = Mock()
    mock_detector = Mock()
    mock_classifier = Mock()
    mock_smoother = Mock()
    # Not testing real code, just mocks! WRONG!
    result = orchestrate(mock_video, mock_detector, mock_classifier, mock_smoother)
```

**GOOD:**
```python
def test_with_real_objects():
    # Use real objects when practical
    detector = BallDetector(test_config)
    classifier = PhaseClassifier(test_config)

    # Mock only expensive/external dependencies
    mock_video = Mock(spec=VideoProcessor)
    mock_video.extract_frames.return_value = [(0, 0.0, test_frame)]

    result = orchestrate(mock_video, detector, classifier)
```

**Why:** Over-mocking tests nothing real. Mock only slow/external dependencies.

---

### ❌ 7. No Assertions (Smoke Tests)

**BAD:**
```python
def test_runs_without_error():
    # Just running code doesn't test behavior!
    detector = BallDetector()
    detector.detect(frame)
    # No assertion! WRONG!
```

**GOOD:**
```python
def test_detects_ball_correctly():
    detector = BallDetector()
    ball = detector.detect(frame)

    # Explicit assertions
    assert ball is not None, "Should detect ball in frame"
    assert 0 <= ball.x < frame.shape[1], "Ball x should be in frame"
    assert ball.confidence > 0.5, "Detection confidence should be high"
```

**Why:** Tests must verify behavior, not just run code.

---

### ❌ 8. Unclear Test Names

**BAD:**
```python
def test_1():  # WRONG!
def test_ball():  # WRONG!
def test_detector():  # WRONG!
```

**GOOD:**
```python
def test_ball_detector_returns_none_when_no_ball_in_frame():
def test_ball_detector_finds_white_ball_in_center():
def test_ball_detector_ignores_similar_colored_objects():
def test_phase_classifier_identifies_yellow_five_bar_zone():
def test_temporal_smoother_removes_phases_shorter_than_one_second():
```

**Why:** Test name should describe what's being tested and expected behavior.

---

## Git Commit Practices

### ALWAYS Make Git Commits

**Commit after:**
1. ✅ Each test passes (RED → GREEN)
2. ✅ Each refactoring (GREEN → REFACTOR)
3. ✅ Each complete feature
4. ✅ Each bug fix

**Commit frequency:** Small, atomic commits > large infrequent commits

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `test:` - Add or modify tests
- `feat:` - New feature
- `fix:` - Bug fix
- `refactor:` - Code refactoring (no behavior change)
- `docs:` - Documentation only
- `style:` - Formatting, missing semicolons, etc.
- `perf:` - Performance improvement
- `chore:` - Build process, dependencies, etc.

### Good Commit Messages

```
test: add test for ball detection with white ball

feat: implement ball detection using color-based method

refactor: extract HSV color conversion to separate method

fix: correct zone boundary calculation for calibration

test: add test for temporal smoothing with short phases

feat: implement temporal smoothing to filter brief transitions
```

### TDD Commit Pattern

```bash
# 1. Write failing test
git add tests/test_detector.py
git commit -m "test: add failing test for ball detection"

# 2. Make test pass
git add src/detector.py
git commit -m "feat: implement basic ball detection"

# 3. Refactor
git add src/detector.py
git commit -m "refactor: extract color threshold logic to config"

# 4. Add more tests for edge cases
git add tests/test_detector.py
git commit -m "test: add test for ball detection with no ball present"

# 5. Handle edge case
git add src/detector.py
git commit -m "fix: handle case when no ball detected in frame"
```

### Commit Anti-Patterns to AVOID

❌ **DON'T:**
- Commit without running tests
- Make huge commits with multiple unrelated changes
- Use vague messages like "fix stuff" or "update code"
- Commit commented-out code or debugging prints
- Commit broken/failing tests

✅ **DO:**
- Run tests before committing (`pytest tests/`)
- Make small, focused commits
- Write clear, descriptive messages
- Clean up before committing
- Ensure tests pass

## Workflow Summary

```
┌─────────────────────────────────────────┐
│  1. Write Failing Test (RED)            │
│     git commit -m "test: ..."           │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  2. Write Minimal Code (GREEN)          │
│     git commit -m "feat: ..."           │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  3. Refactor & Improve (REFACTOR)       │
│     git commit -m "refactor: ..."       │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  4. Repeat for Next Feature             │
└─────────────────────────────────────────┘
```

## Quick Reference

### Before Writing Code:
1. ❓ What behavior am I implementing?
2. ✍️  Write a test that verifies that behavior
3. ▶️  Run test - it should FAIL
4. 💻 Write minimal code to pass the test
5. ✅ Run test - it should PASS
6. 🔄 Refactor if needed
7. 📝 Commit with clear message

### Before Committing:
- [ ] All tests pass (`pytest tests/`)
- [ ] No debugging code left in
- [ ] Clear commit message
- [ ] Focused, atomic change
- [ ] No commented-out code

### Red Flags:
- 🚩 Writing code without a test first
- 🚩 Test that passes immediately (not testing anything new)
- 🚩 Committing broken tests
- 🚩 Large commits mixing multiple features
- 🚩 Tests depending on each other
- 🚩 Testing implementation details instead of behavior

## Running Tests

```bash
# Run all tests
pytest tests/

# Run specific test file
pytest tests/test_detector.py

# Run specific test
pytest tests/test_detector.py::test_ball_detector_finds_ball

# Run with coverage
pytest --cov=src tests/

# Run in watch mode (rerun on file change)
ptw tests/
```

## Remember

> "Test behavior, not implementation"
> "Red, Green, Refactor"
> "Commit early, commit often"
> "Make the change easy, then make the easy change"

---

**These guidelines ensure:**
- ✅ Code works as intended (tests verify behavior)
- ✅ Refactoring is safe (tests catch regressions)
- ✅ Changes are reviewable (small, focused commits)
- ✅ History is clear (good commit messages)
- ✅ Bugs are caught early (TDD catches issues immediately)

---

# Token Discipline

Context is the scarce resource. Every token in the main conversation is
re-sent on every subsequent turn, so pollution compounds. Spend main-context
tokens on decisions; delegate consumption of raw material to subagents.

## Delegation defaults

- **Never read files into main context to "look around."** Use the `scout`
  agent (Haiku, read-only) for any where/how/what-exists question. Fan out
  multiple scouts in parallel for independent questions; you keep the
  conclusions, not the file dumps.
- Read a file directly only when you are about to edit it, and prefer reading
  the relevant slice over the whole file.
- Verification, review, and research that produce lots of intermediate output
  belong in subagents (`verifier`, `critic`, `Explore`) whose transcripts are
  discarded — only their final report costs you anything.

## Model and effort matching

- Mechanical or lookup work → Haiku / `effort: low` (the `scout` default).
- Judgment work (specs, review, tricky implementation) → session model.
- Don't pay frontier-model rates to run `grep`.

## Session hygiene

- One task per session. When a task completes, `/clear` and start the next
  from its task file rather than carrying dead context forward.
- Long-running work should be resumable from artifacts on disk (specs, task
  files, notes in `docs/`), never from conversation memory. If a session is
  getting heavy mid-task, write a handoff file and restart from it.
- Don't re-run searches or re-read files already established this session;
  don't paste large command output back into the conversation — summarize it.

## Cheap before expensive

- Critique the spec before implementing it (a `critic` pass costs ~1% of a
  wrong implementation).
- Make acceptance checks runnable commands, so verification is one cheap
  subagent instead of an interactive debugging spiral.
