# Localization

Answers: what a translator may change, what must survive translation untouched,
and how to add a language end to end.

## The shape of a content file

`content/<locale>.yaml` has two top-level keys:

```yaml
narration:
  Title: "This is the loop, in ninety seconds."
  Outro: "That is the whole cycle."

copy:
  title:
    heading: "The autonomous loop"
    subheading: "An explainer"
  outro:
    heading: "Where to go next"
    links: ["README", "ADR index"]
```

`narration` keys are scene ids and must match `scenes:` in the config (CHK-10).
`copy` is free-form nested structure — objects, arrays, strings, numbers,
booleans. The `Copy` TypeScript interface in `src/generated/content.ts` is
**derived from the default locale's shape**, so adding a field on screen is a YAML
edit and the type arrives with the value. A translation that drops or renames a
key fails to compile rather than rendering an empty box.

Every locale's file has the same shape as the default's. That is not a convention;
it is CHK-11.

## Three kinds of string

The single most useful distinction when translating:

| Kind | Example | Treatment | Check |
| --- | --- | --- | --- |
| **Printed literal** | `GOAL MET`, `session_token`, `exit 1` | never translated, byte-identical | CHK-15 |
| **Glossary term** | `loop`, `subagent`, `worktree` | stays in the source language inside prose | CHK-16 |
| **Prose** | headings, captions, narration | translated freely, within budget | CHK-14 |

**Printed literals** are strings the system itself prints. Translating one makes
the video contradict the product it explains, and the viewer who greps for what
they saw on screen finds nothing. Declare them:

```yaml
printedLiterals:
  - "GOAL MET"
  - "ORACLE TOO WEAK"
  - "session_token"
```

CHK-15 finds every default-locale string containing a declared literal and
requires the same literal, verbatim, in every translation of that string.

**Glossary terms** are domain vocabulary the audience says in the source language
regardless — and a translated term stops being greppable in the docs, the logs and
the issue tracker.

```yaml
glossary: [loop, subagent, worktree, oracle]
```

CHK-16 matches **whole words, case-insensitively, in prose only**, and skips any
string that is itself a declared printed literal. Those bounds are the fix for a
check that was previously vacuous: the reference project's version stringified the
whole copy object — including code literals no translator ever touches — and
matched without word boundaries, so `turn` was satisfied by `returns` and the
check could never fail.

If a term genuinely should be translated, remove it from `glossary`. Do not leave
it listed and untranslated in the file; the list is the record of the decision.

## Expansion budget

```yaml
locales:
  list:
    - code: vi
      expansionFactor: 1.35
```

CHK-14 fails a string longer than `len(default) × expansionFactor` runes. It is a
**proxy for visual overflow, not a proof of it** — the budget is derived from the
default locale so nobody maintains a per-slot table, and the only tuned number is
one factor per language.

When it fails: shorten the line first. Raise `expansionFactor` only after actually
looking at the frame in Studio and confirming the layout holds. Raising it to
silence the check is how a caption ends up off the frame edge in the one language
nobody on the team reads.

The default locale's own factor is `1.0` — it is its own baseline.

## NFC

**CHK-17 fails any text that is not in Unicode Normalization Form C.** Ten lines
of check, and the highest value per line in the set.

`ố` can be written as one codepoint (U+1ED1) or as `ô` + U+0301. The two look
identical in a review, in a terminal, and in a diff. They do not behave
identically on screen: the decomposed spelling depends on the font's mark
attachment tables, which a subset webfont need not carry — and the decomposed form
also passes a per-codepoint coverage check on any font holding the base and the
mark separately, which says nothing about whether the pair renders.

So NFC is asserted first, and only then is font coverage a meaningful question.
See `fonts.md`.

To fix: normalize the file. Most editors have a command; `python3 -c "import
unicodedata,sys; sys.stdout.write(unicodedata.normalize('NFC',
sys.stdin.read()))"` will do it. Watch for text pasted from macOS filesystem
paths, which are NFD by convention.

## Adding a language, end to end

1. **Config** — add the locale and name its face:

   ```yaml
   locales:
     default: en
     list:
       - code: en
         label: English
         charsPerSecond: 16.17
         expansionFactor: 1.0
         font: body
         requiredSample: "The quick brown fox jumps over the lazy dog"
       - code: vi
         label: Tiếng Việt
         charsPerSecond: 15.55
         expansionFactor: 1.35
         font: bodyVi
         requiredSample: "tuyên bố ố ầ ế ấ ư ơ đ ọ"
   ```

   `requiredSample` must include the script's **hardest stacked marks**, not a
   pangram of easy letters. It is the input to CHK-18, and a sample that only
   contains characters every font has proves nothing.

2. **Font** — add the face under `fonts:` and point the locale at it. A face that
   covers latin says nothing about the script you are adding. Read `fonts.md`
   before choosing.

   Also add the family to `bodyFamilyFor` in `src/fonts.ts`. That map is keyed by
   `Locale`, so adding a language **breaks that file until a face is named** —
   which is exactly the right place to be stopped.

3. **Voice** — add `tts.voices.<code>` with a `voiceId` and a `model` that is
   cleared for the language. `nv` refuses a denied *or unlisted* model; see
   `tts-providers.md`. Clearing a new model means listening to it with someone who
   speaks the language.

4. **Content** — copy `content/en.yaml` to `content/<code>.yaml` and translate.
   Keep the structure identical, including **array lengths**: TypeScript makes a
   missing key a compile error, but a list of five gates in one language and three
   in another satisfies the same type and renders a short table in silence. That
   is CHK-11's whole reason for walking the structure recursively.

5. **Synthesize and check**:

   ```bash
   nv voiceover vi
   nv validate
   ```

6. **Render** — the new composition is `<VideoId>-<locale>` (the default locale
   keeps the bare id, so the composition anyone renders by habit stays where it
   was). `nv sync` derives it; `Root.tsx` maps over `LOCALES × TIMELINE`, so
   nothing in TypeScript changes.

## What a second language costs, and why so little

Nothing about the timeline is re-tuned. Scene lengths come from the new
narration's own measurements, and every reveal inside a scene is cued as a
fraction of `durationInFrames`, so a line that runs 12% longer carries its cues
with it. That is the single design decision that makes translation cheap — and
CHK-22 is what keeps it true, by refusing an absolute frame count in cue position.

The reference project wrote all eight of its scene files **twice**, because
localization was bolted on after the fact rather than being structural.
