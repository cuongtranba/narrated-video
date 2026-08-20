# Borrowing from general Three.js material

Answers: which parts of ordinary Three.js advice apply inside this kit, which
parts will pass review and fail the render, and how to translate the rest.

Derived from surveying [cloudai-x/threejs-skills](https://github.com/cloudai-x/threejs-skills)
(MIT), a ten-topic Three.js knowledge set. Nothing is vendored here, for a
reason worth stating: that material — like almost all Three.js writing — targets
an **interactive browser canvas**, and this kit renders **headless, frame by
frame, in parallel tabs**. Those are close to opposite environments, and the
advice that does not survive the crossing fails *silently*.

A census of that repo against this kit's gate:

| Pattern | Occurrences | Here |
| --- | --- | --- |
| `addEventListener` | 26 | no user exists in a render |
| `Math.random` | 13 | **CHK-28 fails** — differs per capture tab |
| `EffectComposer` | 10 | needs its own render loop |
| `Raycaster` | 9 | pointer interaction, meaningless |
| `requestAnimationFrame` | 6 | **CHK-28 fails** |
| `AnimationMixer` | 6 | wall-clock driven |
| `OrbitControls` | 5 | interactive |
| `new THREE.Clock` | 3 | **CHK-28 fails** |

It is also vanilla `THREE`, which a scene may not import at all (**CHK-37**).
Copied in unchanged it would teach patterns the gate rejects — and the
clock-driven ones render perfectly per frame while differing between tabs, so
the failure survives a spot-check.

## What transfers, and how

| Topic | Verdict | Translation |
| --- | --- | --- |
| Geometry | **transfers as-is** | `<boxGeometry>`, `<cylinderGeometry>` and friends are declarative and deterministic. Compose them into `Datastore`/`Rack`-style objects. |
| Materials | **transfers with one change** | Colour comes from `THEME_3D`, never a literal — three cannot parse OKLCH and CHK-41 refuses literals in scenes. |
| Lighting | **mostly owned by `<Space>`** | It supplies key, fill and ambient tuned for the dark theme. Add lights inside `<Space>` only for an effect the default rig cannot give. |
| Shaders | **transfers with one change** | A `uTime` uniform must be fed `useCurrentFrame()`, never a `Clock`. Frames are captured out of order. |
| Textures | **needs the asset rules** | Under `public/`, referenced with `staticFile()`, and the frame held while it loads. See CHK-31 and CHK-35. |
| Loaders / glTF | **does not work yet** | See below. |
| Animation | **rewrite entirely** | `AnimationMixer` and keyframe playback are clock-driven. Every transform here is a pure function of `useCurrentFrame()`; a rotation is `frame * rate`, not a mixer update. |
| Post-processing | **not available** | `EffectComposer` replaces the render loop that `@remotion/three` owns. |
| Interaction | **discard** | Raycasting, `OrbitControls` and pointer events have no meaning in a render. The equivalent of "look around" is a `turntable`; the equivalent of "focus" is a camera move or `Focus`. |
| Fundamentals | **discard** | Scene/camera/renderer setup is `<Space>`'s job, and CHK-37 forbids a scene from doing it. |

## glTF loading — attempted, not working

Loading real models would be the biggest single capability gain here: it is the
difference between composing a datastore out of cylinders and importing one. I
built a `<Model>` component for it and **could not get it to render**, so it is
not shipped rather than shipped broken.

What was tried, and what each attempt ruled out:

1. `GLTFLoader` + `delayRender`/`continueRender`, releasing the handle in the
   load callback → empty stage, exit 0.
2. Removing the `useEffect` cleanup that released the handle early → no change,
   so the premature release was not the cause.
3. Releasing on the commit that mounts the model rather than inside the load
   callback → no change, so it is not a state-flush race.
4. `invalidate()` from `useThree` after the model mounts → no change.

A control mesh rendered in the same `<Space>` throughout (≈50,000 pixels), and
the turntable animates, so the canvas and per-frame repaint are fine. A
known-good Khronos `Box.gltf` failed identically to a hand-written fixture, so
the asset was not the problem either.

The remaining hypothesis is that `@remotion/three` paints the canvas once per
frame and an object entering the scene graph **asynchronously** never reaches a
paint — meaning the model must be resolved *before* the canvas renders, not
inside it. The likely shape of a fix is loading above `<Space>` on the DOM side
and passing the resolved object in, or a Suspense-based `useLoader` that r3f
resolves before its first paint. Neither is implemented.

Until then, build objects from geometry — `references/3d.md` § the object
vocabulary — which is verified working.
