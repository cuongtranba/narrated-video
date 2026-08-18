import { registerRoot } from "remotion"

import { RemotionRoot } from "./Root"

registerRoot(RemotionRoot)

export { Count, Emphasis, Focus, Stagger, Trace } from "./components/motion"
export type { FocusOn, FocusTransformResult } from "./motion-math"
export { countValue, focusTransform, staggerOffset } from "./motion-math"
