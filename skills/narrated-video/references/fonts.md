# Fonts

Answers: how to know a font can actually draw a language before the video ships,
and what that knowledge does not cover.

## The failure this exists for

A Vietnamese localization shipped with `tuyên bố` rendered as `tuyên bô`.

No missing-glyph box. No warning. No console error. The webfont simply had no
glyph for U+1ED1 and drew nothing at all. It was caught by cropping a
full-resolution still and reading it with a native eye — after the cut had been
reviewed and approved.

The obvious defence (trust a declared `subsets: [latin, vietnamese]` list against
a bundled range table) is **unsound**, and this was verified against the actual
bytes. Parsing the offending `body-regular.woff2` — woff2 table walk, brotli,
cmap format 4, offsets reconciled exactly:

| codepoint | in the font's cmap? |
| --- | --- |
| `ọ` U+1ECD | **yes** |
| `đ` U+0111 | **yes** |
| combining U+0301, U+0303, U+0323 | **yes** |
| `ố` U+1ED1 | **NO** |
| `ầ` U+1EA7 | **NO** |
| `ế` U+1EBF | **NO** |
| `ấ` U+1EA5 | **NO** |
| `ư` U+01B0 | **NO** |
| `ơ` U+01A1 | **NO** |

`ọ` sits squarely inside Google's `vietnamese` range (U+1EA0–1EF9). **Any coarse
"does this font do Vietnamese" test says yes.** And a declared-subsets check takes
as its input the very belief under test: whoever shipped the reference believed
the face handled Vietnamese, would have typed `subsets: [latin, vietnamese]`, and
would have sailed straight past the bug.

The font file is committed at `internal/fonts/testdata/body-regular.woff2` and
that table is a test fixture, so the parser can never quietly stop detecting it.

## Three layers

### 1. NFC first (CHK-17)

Assert the text is composed before asking about coverage. `b` + U+00F4 + U+0301
and `b` + U+1ED1 are the same word, but the decomposed spelling passes a
per-codepoint check on any font holding U+00F4 and U+0301 separately — while what
actually gets drawn then depends on mark-attachment tables a subset webfont need
not carry. Normalizing first removes the whole combining-sequence case from the
input. See `localization.md`.

### 2. Google fonts: verify the *request*

For a Google font there is no file on disk at validate time, and the mistake worth
catching is a different one: `@remotion/google-fonts` only injects an `@font-face`
for the subsets you ask for. Forget `"vietnamese"` and the browser falls back
silently — correct-looking text in the wrong face, or no diacritics at all.

So CHK-18 checks the **declared subsets against the standard published ranges**
and reports which characters of `requiredSample` they do not cover:

```yaml
fonts:
  bodyVi:
    kind: google
    family: Be Vietnam Pro
    importName: BeVietnamPro
    subsets: [latin, vietnamese]     # forgetting `vietnamese` is the bug
    weights: ["400", "600", "700"]
```

Known subsets: `latin`, `latin-ext`, `vietnamese`, `cyrillic`, `cyrillic-ext`,
`greek`, `greek-ext`. An unrecognised subset name is reported rather than ignored.

No per-family range table is bundled. It would be a stale duplicate of what Google
publishes, and it would be *wrong* for `latin` on some families — Be Vietnam Pro's
own latin range carries extra U+0304, U+0308, U+0329 beyond the standard spec.
The ranges here are used only to judge what was **requested**, never to conclude
what a font contains.

`subsets` in the config and the `loadFont(...)` call in `src/fonts.ts` must name
the same subsets. The config is what `nv` checks; `src/fonts.ts` is what the
browser gets. They are two halves of one decision.

### 3. Local fonts: read the real cmap

For `kind: local`, the file is parsed and its cmap read per codepoint. No declared
subset is trusted, because that is the belief under test.

```yaml
fonts:
  bodyVi:
    kind: local
    family: My Face
    files:
      - path: fonts/my-face-regular.woff2   # relative to public/
        weight: "400"
        style: normal
```

This is feasible in Go because **woff2 never transforms `cmap`** — the format
defines transforms only for `glyf`, `loca` and `hmtx`. After one brotli
decompression of the single shared stream, the cmap sits verbatim at its
accumulated offset; reading coverage needs no glyph reconstruction at all.

Supported: woff2, ttf, otf; cmap formats 4 and 12. Refused with a clear message:
woff1 (convert it), and font collections (`ttcf` — coverage is a property of one
face, so extract the face you mean). A codepoint mapped to glyph 0 (`.notdef`) is
treated as **absent**, not as coverage of a blank.

One implementation trap worth knowing if you touch `internal/fonts/cmap.go`: the
woff2 transform-flag semantics are **inverted** for `glyf` and `loca` (version 3
is the null transform there, so 0 means transformed; everywhere else 0 is the null
transform). Reading it backwards consumes the wrong number of varints and shifts
every subsequent table offset, after which the parse reads plausible garbage
rather than failing.

## `requiredSample` is the input, so it decides what is proven

```yaml
- code: vi
  font: bodyVi
  requiredSample: "tuyên bố ố ầ ế ấ ư ơ đ ọ"
```

Include the script's hardest stacked marks. A pangram of easy letters proves
nothing — `ọ` was present in the font that shipped the bug. A locale with no
`requiredSample` fails CHK-18 outright: nothing about its font can be checked, and
silently passing would be the worst of both.

## The honest limit

**A cmap hit proves that a codepoint maps to *a* glyph id. It does not prove the
glyph is non-empty, and it says nothing about mark attachment.**

A font could map U+1ED1 to a blank glyph, or carry the composed codepoint without
the GPOS mark-positioning rules that place a tone mark correctly over a modified
vowel. Both would pass CHK-18 and both would look wrong.

What this layer buys is the elimination of the specific silent failure that
actually shipped — the missing codepoint — plus a guarantee that the question is
being asked about composed text. It is not a rendering proof. **For a new script,
render one still and have someone who reads the language look at it.** That is one
`bun run still` and five minutes, once per language, forever.

## Loading, and why the first frame waits

`src/fonts.ts` wraps font loading in `delayRender` / `continueRender`:

```ts
const fontsReady = delayRender("Loading fonts")
void Promise.all([body.waitUntilDone(), mono.waitUntilDone()])
  .then(() => continueRender(fontsReady))
  .catch((error: unknown) => cancelRender(error))
```

The reference project fired a bare `void Promise.all(...)` with nothing holding
the render back, so frames could composite before a face resolved — a still, or
the first frames of a cut, would ship in the fallback. `delayRender` is the only
thing that makes typography deterministic.

`bodyFamilyFor` is keyed by `Locale` on purpose: adding a language to `LOCALES`
breaks `src/fonts.ts` until someone names a face for it. Being stopped there is
the point.

## Committing a local font

Local font files live under `public/` and are committed. They are usually well
under a megabyte subsetted; if a face is large, see the size guidance in
`troubleshooting.md` before reaching for Git LFS.
