import { Mono, Reveal, SIZE } from "../components/primitives"
import { Stage } from "../components/stage"
import { useCopy } from "../content/use-copy"
import { THEME } from "../generated/theme"
import { at, beatSpans } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { links: 0.4 }

export const Scene: SceneComponent = ({ durationInFrames }) => {
  const { outro } = useCopy()

  const linksAt = at(CUE.links, durationInFrames)
  // Equal weights: the links carry equal weight, so they share the rest of the
  // scene evenly. A list whose last item is the point would pass `[1, 1, 3]`
  // and stay proportional in every language.
  const spans = beatSpans(
    outro.links.map(() => 1),
    durationInFrames - linksAt,
  )

  return (
    <Stage name="Outro">
      <div
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          gap: 40,
        }}
      >
        <Reveal name="Heading" at={0}>
          <h1
            style={{
              margin: 0,
              fontWeight: 700,
              fontSize: SIZE.display,
              letterSpacing: "-0.02em",
              lineHeight: 1.08,
              maxWidth: 1500,
            }}
          >
            {outro.heading}
          </h1>
        </Reveal>

        <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
          {outro.links.map((link, index) => (
            <Reveal key={link} name={`Link ${link}`} at={linksAt + (spans[index]?.from ?? 0)}>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 20 }}>
                <span
                  style={{
                    width: 12,
                    height: 12,
                    borderRadius: 999,
                    backgroundColor: THEME.accent,
                  }}
                />
                <Mono color={THEME.foreground}>{link}</Mono>
              </span>
            </Reveal>
          ))}
        </div>
      </div>
    </Stage>
  )
}
