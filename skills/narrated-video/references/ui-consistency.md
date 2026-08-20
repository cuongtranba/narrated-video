# UI consistency and the contrast gate

Answers: what makes two scenes look like the same video, which of those rules a
machine can hold you to, and what to do when the gate says no.

A video has one advantage over an app — nobody can resize it — and one
disadvantage that swallows the advantage whole: **nobody can adjust it either.**
There is no zoom, no reader mode, no user stylesheet, no "view original". A
frame that is hard to read is hard to read forever. That is why contrast here is
a gate rather than a lint.

## The floor, and who enforces it

| Rule | Enforced by |
| --- | --- |
| Body and heading colours clear 4.5:1 against what they sit on | **CHK-39**, hard fail |
| The accent clears 3:1 wherever it carries meaning | **CHK-40**, hard fail |
| Colour comes from the theme, never a literal | **CHK-41**, hard fail |
| Type is sized from the shared scale | **CHK-42**, hard fail |
| One safe area, one entrance curve | the components, by construction |
| Colour never carries meaning alone | you — see below |
| One accent, used for one job | CHK-41 keeps it in the palette; the count is yours |

Four of those are machine-checkable and are checked, compiled into `nv` rather
than written down — they hold on a fresh clone with no skill loaded. The rest
are yours, and the sections below are what "consistent" means when no check is
watching.

## Colour never travels alone

Every state that means something must survive being printed in greyscale, shown
on a projector with the contrast washed out, or watched by someone who cannot
separate your two tints. The kit is built so the easy path already complies:

- `Pill` pairs its tint with a **label** and a dot. There is no colour-only pill.
- `Emphasis` never tints without also changing shape or style — `tint` adds
  italic, `underline` draws a line, `weight` changes the weight.
- `Diagram`'s `subject` dims by **opacity**, which survives greyscale, rather
  than recolouring the subject.

When you add something new, carry the same rule: a red edge and a grey edge are
one difference; a red *dashed* edge and a grey *solid* edge are two.

## The two-step rule for surfaces

An object on a surface needs two steps of tone between them, not one. The kit
ships `background` → `surface` as those two steps, and the diagram canvas sits
at `background` precisely so its nodes at `surface` read as objects on it.

This is worth stating because the failure is quiet and geometric: a panel and
its cards at the same colour, separated by a 1.2:1 hairline, looks fine on the
laptop you built it on and disappears on anything else. If you find yourself
adding a border to make a shape visible, check the fill first.

## One accent, one job

The shipped theme has exactly one accent, and it already has work: the `Rule`
that marks where the voice starts, the underline under a defined term, the
stroke of a diagram edge, the emissive core of a 3D subject. Adding a second
accent for a new scene is the fastest way to make a cut look like two videos.

If a scene needs to distinguish two things, reach for the tools that do not
spend colour: opacity (`subject`), motion (`flowing: false` on the edge that
carries nothing), shape (`Pill` versus `Mono`), or position.

If it genuinely needs a new colour, add it to `theme` rather than to the scene.
CHK-41 enforces that, and the reason is not tidiness: a literal is invisible to
CHK-39, so a colour written into a scene is a colour whose contrast nobody ever
scores.

## Motion consistency

The same event should move the same way every time it appears:

- Anything **entering** uses `Reveal`, so the whole video shares one curve.
- Anything **carrying** uses `Flow` or `Beam`, so traffic looks like traffic in
  both 2D and 3D.
- Anything **drawing itself** uses `Trace` or `Rule`.

A scene that invents its own entrance is the visual equivalent of a font change.

## When the gate fails

`nv validate` prints the pair, the ratio it measured, and the grade:

```
FAIL CHK-39  text colours meet WCAG AA contrast against what they sit on
  theme.muted on theme.background   3.90:1 (AA-large) — secondary paragraphs on the Stage needs at least 4.5:1
  fix: raise the lightness gap between the pair named above — see references/ui-consistency.md
```

Raise the lightness gap. In OKLCH the first number is perceptual lightness, so
moving `muted` from `oklch(58% …)` to `oklch(70% …)` does roughly what it looks
like it does, and the hue and chroma stay where you put them — which is the
reason the config is written in OKLCH rather than hex.

Do not fix it by deleting the key. An absent colour is unscored, so removing
`muted` turns a red check green and leaves the frame exactly as unreadable as it
was. That is the `expansionFactor` mistake in a different costume, and
`references/validate-checks.md` explains why the gate treats it as one.

There is no `--force`. A flag that waved this through would cost `nv validate`'s
exit code the one property it has.

## What the gate cannot see

It reads the palette, not the frame. It cannot tell you that a heading overlaps
a diagram, that a translation wrapped to three lines, that a `Stagger` finished
long before the sentence did, or that two scenes disagree about where the
subject sits. Those need a frame — render a still and look at it. `SKILL.md`
§ Look at the frame is the loop.
