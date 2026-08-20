# The brief

Depth behind SKILL.md § The brief: the questions worth asking, how to push when
an answer is vague, what the file holds, and a worked example of the same beat
written from an empty brief and a full one.

- [Why the questions come first](#why-the-questions-come-first)
- [What to read before you ask](#what-to-read-before-you-ask)
- [The question bank](#the-question-bank)
- [Pushing on a vague answer](#pushing-on-a-vague-answer)
- [When they will not engage](#when-they-will-not-engage)
- [The file](#the-file)
- [From brief to config](#from-brief-to-config)
- [Worked example](#worked-example)

## Why the questions come first

Narration is prose, and a model asked for prose about a half-known subject will
produce fluent prose about a half-known subject. Nothing downstream catches it.
The gate reads no prose, the renderer reads no meaning, and a wrong sentence
costs exactly as many frames as a right one — so the error survives every stage
and arrives at the viewer sounding like the parts that are true.

That is the whole argument for asking first. Not diligence, and not ceremony:
the interview is the only stage in this pipeline where a false sentence can
still be caught, because it is the only stage where someone who knows the
subject is still in the room.

The second argument is smaller but it is the one you will feel: the answers are
the material. A video assembled from what a model can infer about a topic
contains, by construction, only what is inferable about that topic — which is
what generic sounds like. Everything that makes a video worth the eleven minutes
of someone's attention comes from the person who built the thing, and none of it
is in the README.

## What to read before you ask

Read first. Not to skip the interview — to earn a sharper one.

A question whose answer is in a file you could have opened spends the user's
patience on your homework, and they answer it briefly because it bores them.
Twenty of those and they stop reading your questions properly, which is when the
interview stops working.

Worth reading before asking, when it exists: the README, the ADR or design doc,
the changelog entry, the actual code path the video is about, the issue thread
where it was argued about. The thread is often the best of these, because it
contains the objections — and the objections are the part no document keeps.

Then ask about what reading cannot tell you:

- what is **out of date** — a README describes the version that was true when
  someone last cared about the README
- what is **deliberate** — code shows the choice, not the alternatives rejected
- what is **weighted** — a doc lists eight features; only the user knows which
  one is the reason the thing exists
- what is **embarrassing** — the limitation, the workaround, the part that is
  slow. Never written down, always the most useful thing in the video
- who is **watching** — nothing on disk knows this, and it determines everything

## The question bank

Not a script. Pick what the video needs, ask in one batch, and stop when the
remaining unknowns would not change the cut. Six or seven is usually plenty;
past that you are gathering material for a different video.

Each question below is paired with what its absence does to the output, because
that is what tells you whether you can afford to skip it.

### What it is for

**Who is watching, and what do they do differently afterwards?**
Without it, narration addresses nobody and hedges every claim to cover everyone.
The cut has no ending, because "what they do differently" *is* the ending.

**What do they believe right now that is wrong?**
The highest-yield question in the bank. An answer gives you the cut's spine: the
video exists to move someone from one belief to another, and that is a shape —
setup, turn, consequence. Without it you inherit the source document's shape,
which is reference order, which is a list. This is the question that decides
whether the video is an argument or a table of contents.

**If they remember one sentence, which one?**
Forces a priority where the source document has none. That sentence usually
wants to be the closing beat; sometimes it is so good it becomes the opening.

### What is true

**Which claims can you point at, and where?**
Ask for the artifact — the benchmark, the log line, the commit, the file you can
open. Sort the answers into *sourced* and *impression*, and write both into the
brief under those headings. An impression is not disqualified; it is disqualified
from being stated as a measurement. "It felt much faster" is honest narration.
"Roughly forty percent faster" is not, unless someone measured it.

**What does it not do?**
Two things at once. It stops the video overclaiming into the first question a
viewer will ask, and it is often the most interesting beat in the cut, because
a limit is specific in a way a capability is not. Videos that name their limits
are believed about everything else.

### What is worth watching

**What surprised you?**
The single best anti-generic question there is. A surprise cannot be inferred
from the topic — that is what makes it a surprise — so it is guaranteed to be
material a model could not have produced alone. It is also, reliably, the beat
viewers remember.

**What has to be seen rather than said?**
Names the beats that want `flow` or `space` before you have started writing
paragraphs. Structure, movement, before-and-after, scale. Ask it early: it is
much cheaper to plan a diagram than to convert a finished text scene into one.

## Pushing on a vague answer

This is the part that does the work, and it is the part that gets skipped,
because a vague answer *feels* like an answer and moving on feels polite.

A vague answer is an unanswered question. Accepting one does not lose you the
information — it is worse than that. The gap gets filled anyway, downstream,
with a sentence in the right register and the wrong content, and that sentence
is now indistinguishable from the sourced ones.

So push once. Not interrogation — one concrete follow-up, and the follow-up asks
for a thing rather than a rephrasing. "Can you say more about that" invites a
longer vague answer. "Faster than what, on what input?" invites a number.

| They say | What it becomes if you accept it | Ask instead |
| --- | --- | --- |
| "it's faster" | "significantly faster" | faster than what, measured how, on what input? |
| "it's easier to use" | "a seamless experience" | what did someone have to do before, and what do they type now? |
| "for developers" / "for our users" | narration aimed at nobody, hedged to cover everyone | which one — someone deciding whether to adopt it, or someone already using it? Those are opposite videos |
| "it's more reliable" | "enterprise-grade reliability" | what used to break, how often, and what happens now instead? |
| "it uses AI" | "leverages advanced AI" | what does it do that a lookup table could not? |
| "it saves time" | "boosts productivity" | whose time, doing what, and how much of it? |
| "it's more flexible" | "highly configurable" | what did someone want to do that they couldn't? |
| "it just works" | "works out of the box" | what is the first command, and what does it print? |

The pattern under the table: every vague answer is an abstraction over a
concrete event that actually happened. The follow-up asks for the event. Someone
was slow at something, or something broke on a Tuesday, or a person typed nine
commands and now types one. Those are what a video can show.

If a second push does not land, stop. Write it into the brief as an impression
and move on — the brief's job is to record what is known, including the shape of
what is not.

## When they will not engage

Some people want a video, not a conversation. Read it correctly rather than
grinding: short answers to your first batch usually means the questions were too
abstract, not that the person is unwilling. Retry once with the sharpest,
most concrete question you have, ideally one built from something you read.

If it is still not landing, say plainly what the consequence is and let them
choose — something close to: *I can draft this from the README alone, but then
the specifics are limited to what it says, and anything the video claims beyond
that would be me guessing. Want the narrow honest version, or five minutes on
the two questions that would open it up?*

That is not a refusal, and it is not a lecture. It is the trade stated once, in
their terms, with both options genuinely on offer. Most people pick the five
minutes. The ones who do not have told you something real about how much the
video matters, and the narrow honest version is a legitimate deliverable: fewer
claims, every one of them sourced, no filler standing in for material that was
never gathered.

What is not on offer is the third option where you write the confident version
anyway from inference. That one has no honest label to put in the brief.

### Nobody in the room at all

Distinct from someone declining to answer: a scheduled run, a queued job, a
request whose author has gone. There is no trade to offer and no one to offer it
to, so the narrow honest version is not a choice — it is the only deliverable
available, and SKILL.md § When nobody can answer is the rule.

One failure is worth naming because it looks like diligence and is not. Marking
an invented specific `[ASSUMED]` in a yaml comment feels like a caveat, but
comments do not render: the video says the number out loud in the same voice as
the sourced ones, and the marker is visible only to whoever opens the file. The
viewer — the only person the caveat was for — never sees it.

The test is whether the caveat survives into the artifact. A line in `## Open`
survives, because the sentence never gets written. A comment beside a written
sentence does not.

Where a placeholder genuinely has to exist so the cut can be measured end to
end, keep it out of the spoken lines: a scene can hold its shape with a heading
and a `Pill` reading `unconfirmed` while its narration says only what is
sourced. That is a draft a reviewer can correct. A fluent invented paragraph
with a comment above it is a draft a reviewer approves.

## The file

`brief.md` at the project root, beside `video.config.yaml`. Written once the
answers are in and updated when they change — it is the record the narration is
written from, and the thing a reviewer checks the narration against.

It holds **facts and intent, never narration lines.** Narration lives in
`content/<locale>.yaml` and nowhere else, for the same reason `nv script`
generates the readable script instead of letting you keep one: two files holding
the same sentences agree until the first edit, and the edit always comes.

The brief holds what is true. The yaml holds what is said. A reviewer reads one
against the other, which is only possible while they are different documents.

```markdown
# Brief — <video id>

> Written WITHOUT the interview — delete this block once it is answered.
> Nothing below came from the person who asked. Everything under `## Sourced`
> was read out of a file whose path is named; everything a person would have
> supplied is under `## Open`, and nothing there became a claim.

## Subject
The artifact this video is about, named exactly — a path, a service, a PR.
Then the candidates ruled out and why, because if this line is wrong the cut is
a rewrite rather than an edit. Rule them out on an axis you checked, not on one
that merely sounds distinguishing.
- src/<the file you actually read> (<size>, landed <date>, <PR>)
- not src/<the near miss> — <the difference you verified, and where you saw it>

## Audience
Who is watching, and where they are when they watch it.

## They currently think
The belief the video moves them off.

## Afterwards they
The one thing they do, decide, or understand differently. This is the ending.

## The one sentence
If they keep one, this.

## Sourced
Claims with an artifact behind them. Name the artifact.
- 374 tool calls, 111 min, $38.32 — session log, README.md:76
- ~16 chars/sec speaking rate — measured, references/timing-model.md

## Impressions
True as experience, not as measurement. Narration may say these as experience.
- the second language felt nearly free once cues were fractions

## Limits
What it does not do. Say these out loud in the cut.
- Windows unsupported
- the gate reads no pixels, so nothing catches a heading overlapping a diagram

## Surprises
- a TTS model returns HTTP 200 and a plausible duration while speaking the tones
  wrong — nothing but a native ear catches it

## Has to be seen
Beats that want flow or space rather than a paragraph.
- the sync fan-out — config and manifests into generated/, nothing flowing back

## Open
Asked, not answered. Nothing here may become a stated claim.
- how much of the 111 minutes was rendering vs. authoring
```

`## Open` is the section that earns the file. A gap you have written down is a
gap you will not fill by accident at two in the morning six scenes later; an
unwritten one is just an absence, and absences get filled fluently.

## From brief to config

The brief answers most of what `nv init` and the first config edit need, which
is why it comes first rather than after the scaffold:

| Brief section | Where it lands |
| --- | --- |
| Afterwards they / The one sentence | the closing beat, and often the opening |
| They currently think | the running order — setup, turn, consequence |
| Sourced + Surprises + Limits | the beats themselves; each is a scene candidate |
| Has to be seen | `--kind flow` / `--kind space` at `nv init --scene` |
| Audience | register, and how much is assumed rather than explained |
| the length they asked for | `video.targetDuration` |

On length: `nv status` projects the cut against `targetDuration` from the first
draft on, and the measured rate is about 16 characters a second — so 150 seconds
is roughly 2,400 characters of narration. Budget while the beats are still a
list. If the brief holds nine beats' worth of material and the window is 90
seconds, the material is right and the cut is wrong: pick the beats that carry
`The one sentence` and let the rest go. Trimming a written script costs more,
and trims worse — it removes words, where the choice you actually need is which
beats to drop whole.

## Worked example

Same beat, same subject, two briefs. The subject is this tool's timing model.

**From an empty brief** — everything inferable about the topic, nothing else:

> Our innovative timing system automatically calculates the perfect duration for
> each scene, ensuring your narration and visuals stay perfectly in sync. It's a
> seamless solution that saves you time and eliminates manual work.

Read it against the two tests in SKILL.md. Cover the heading — a heading reading
"Timing" leaves the narration saying *timing is handled automatically and this is
good*, which the heading already said. And every sentence here could have been
written by someone who has never run the tool; "innovative", "perfect",
"seamless" and "eliminates manual work" are the shape of a feature description,
not this feature. Zero specifics, so nothing to be wrong about — which is why
slop is comfortable to write and hard to argue with.

**From a full brief** — sourced numbers, one surprise, one limit:

> Every scene's length is measured from the mp3 that exists — not from what the
> provider said it made. The frame table that came before this was maintained by
> hand, and it was rewritten seven times in one session before anyone noticed the
> real problem: a scene can sit fourteen frames off its audio and still render
> perfectly at every single frame.

The sentence a viewer remembers is the last one, and it is the surprise. It
could not have been inferred from the topic; someone had to have lived it. The
number is sourced, the mechanism is specific enough to be wrong — which is what
makes it worth saying — and the beat now has something for a diagram to show:
the provider's claim discarded, the file measured.

Note what did not happen. The second version is not longer, and it is not more
technical for its own sake. It is the same length carrying information instead
of register.
