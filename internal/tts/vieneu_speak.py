"""Speak one line with VieNeu-TTS. Embedded in `nv` and run by internal/tts/vieneu.go.

Reads one JSON object on stdin:
  model      VieNeu mode, e.g. "v3turbo"
  text       what to say
  out        path to write the wav to
  ref_audio  path to a reference clip to clone, or
  voice      the name of a preset voice
"""
import json
import os
import sys

# onnxruntime refuses a Hugging Face cache laid out with symlinks.
os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS", "1")

from vieneu import Vieneu  # noqa: E402

request = json.load(sys.stdin)
model = Vieneu(mode=request["model"], precision="fp32")

if request.get("ref_audio"):
    audio = model.infer(request["text"], ref_audio=request["ref_audio"])
else:
    audio = model.infer(request["text"], voice=request["voice"])

model.save(audio, request["out"])
