#!/usr/bin/env python3
"""Seed a running OpenAudio API with demo publishers and packages — through the public API only.

Every sample is a small generated WAV (sine tones), uploaded with the resumable upload API, so the
whole publishing pipeline (SHA-256 verification, manifest validation, publish, search indexing) runs.

    ADMIN_EMAILS=demo@openaudio.dev docker compose up -d     # the seeding account must be an admin
    python3 scripts/seed_demo.py --api http://localhost:5173/api/v1

Re-running is safe: existing publishers, packages and versions are skipped.
"""
import argparse
import hashlib
import io
import json
import math
import struct
import sys
import urllib.error
import urllib.request
import wave

NOTE = {"C": 0, "C#": 1, "D": 2, "D#": 3, "E": 4, "F": 5, "F#": 6, "G": 7, "G#": 8, "A": 9, "A#": 10, "B": 11}


def tone_wav(freq, seconds, rate=22050):
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        frames = int(seconds * rate)
        data = bytearray()
        for i in range(frames):
            env = min(1.0, i / 200) * (1 - i / frames)  # click-free attack, linear decay
            data += struct.pack("<h", int(12000 * env * math.sin(2 * math.pi * freq * i / rate)))
        w.writeframes(bytes(data))
    return buf.getvalue()


def freq_of(key, octave=2):
    if not key:
        return 55.0
    return 440.0 * 2 ** ((NOTE[key[0]] - 9) / 12 + (octave - 4))


class API:
    def __init__(self, base):
        self.base, self.token = base.rstrip("/"), None

    def call(self, method, path, body=None, raw=None, ok=(200, 201, 202, 204)):
        headers, data = {}, None
        if raw is not None:
            headers["Content-Type"], data = "application/octet-stream", raw
        elif body is not None:
            headers["Content-Type"], data = "application/json", json.dumps(body).encode()
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        req = urllib.request.Request(self.base + path, data=data, method=method, headers=headers)
        try:
            with urllib.request.urlopen(req) as r:
                txt = r.read()
                return r.status, json.loads(txt) if txt else None
        except urllib.error.HTTPError as e:
            txt = e.read()
            payload = json.loads(txt) if txt else {}
            if e.code in ok:
                return e.code, payload
            return e.code, payload

    def sign_in(self, email, password, name):
        st, res = self.call("POST", "/auth/login", {"email": email, "password": password})
        if st != 200:
            st, res = self.call("POST", "/auth/register", {"email": email, "password": password, "displayName": name})
            if st != 201:
                sys.exit(f"cannot sign in or register {email}: {res}")
        self.token = res["accessToken"]
        return res["user"]

    def upload(self, data, content_type):
        digest = "sha256:" + hashlib.sha256(data).hexdigest()
        st, u = self.call("POST", "/uploads", {"digest": digest, "size": len(data), "contentType": content_type})
        if st not in (200, 201):
            sys.exit(f"upload init failed: {u}")
        for n in range(1, u["partCount"] + 1):
            chunk = data[(n - 1) * u["partSize"]: n * u["partSize"]]
            st, res = self.call("PUT", f"/uploads/{u['id']}/parts/{n}", raw=chunk)
            if st != 200:
                sys.exit(f"part {n} failed: {res}")
        if u["status"] != "done":
            st, res = self.call("POST", f"/uploads/{u['id']}/complete")
            if st != 200:
                sys.exit(f"complete failed: {res}")
        return digest


PUBLISHERS = [("acme", "Acme Audio"), ("tapeworks", "Tapeworks"), ("lattice", "Lattice DSP"), ("fieldnotes", "Field Notes Collective")]

CC_BY = {"content": "CC-BY-4.0", "commercialUse": True, "attributionRequired": True}
CC0 = {"content": "CC0-1.0", "commercialUse": True, "attributionRequired": False}
MIT = {"content": "MIT", "commercialUse": True, "attributionRequired": True}


def module(cls, accepts, produces, placement, categories):
    return {"moduleClass": cls, "categories": categories,
            "capabilities": {"accepts": accepts, "produces": produces, "placement": placement, "bypassable": True}}


# (publisher, slug, name, kind, version, description, license, taxonomy, musical, samples, extra)
PACKAGES = [
    ("acme", "deep-house-toolkit", "Deep House Toolkit", "sample-pack", "1.2.0",
     "Deep house drums, bass loops and textures recorded through a vintage tape chain.", CC_BY,
     {"genres": {"primary": "deep-house", "secondary": ["tech-house"]}, "instruments": ["drums", "bass", "percussion"],
      "moods": ["warm", "hypnotic"], "useCases": ["club", "studio-production"], "energy": 0.62},
     {"bpm": {"value": 124, "range": [122, 126], "sync": True}, "key": {"tonic": "F", "mode": "minor", "camelot": "4A"}, "timeSignature": "4/4"},
     [("loops/bass/rolling-bass-124.wav", "Rolling Bass", "loop", "bass", "bass", 1.9), ("loops/drums/shuffle-groove-124.wav", "Shuffle Groove", "loop", "drums", "drums", 1.9),
      ("one-shots/kick/tape-kick.wav", "Tape Kick", "one-shot", "kick", "drums", 0.4), ("one-shots/hats/dusty-hat.wav", "Dusty Hat", "one-shot", "hi-hat", "drums", 0.2)],
     {"tags": {"free": ["analog", "tape", "rolling"]}}),
    ("tapeworks", "warm-tape-drums", "Warm Tape Drums", "sample-pack", "2.0.1",
     "Acoustic and drum-machine hits bounced to 1/4\" tape. Saturated kicks, dusty snares and swung hat loops.", CC0,
     {"genres": {"primary": "lo-fi", "secondary": ["hip-hop"]}, "instruments": ["drums", "kick", "snare"], "moods": ["warm", "melancholic"], "energy": 0.4},
     {"bpm": {"value": 88, "range": [80, 92]}, "key": {"tonic": "A", "mode": "minor"}},
     [("loops/boom-bap-88.wav", "Boom Bap Loop", "loop", "drums", "drums", 2.7), ("one-shots/snare-dust.wav", "Dust Snare", "one-shot", "snare", "drums", 0.3)],
     {"tags": {"free": ["dusty", "tape", "boom-bap"]}}),
    ("acme", "liquid-dnb-essentials", "Liquid DnB Essentials", "sample-pack", "1.4.2",
     "Rolling breaks, reese basses and lush pads for liquid drum & bass.", CC_BY,
     {"genres": {"primary": "drum-and-bass", "secondary": []}, "instruments": ["drums", "bass", "pad"], "moods": ["uplifting", "dreamy"], "energy": 0.75},
     {"bpm": {"value": 174}, "key": {"tonic": "E", "mode": "minor"}},
     [("loops/breaks/liquid-break-174.wav", "Liquid Break", "loop", "drums", "drums", 1.4), ("loops/bass/reese-174.wav", "Reese", "loop", "bass", "bass", 1.4),
      ("one-shots/pads/air-pad.wav", "Air Pad", "one-shot", "pad", "pad", 3.0)],
     {"tags": {"free": ["breaks", "reese", "liquid"]}}),
    ("fieldnotes", "nordic-field-recordings", "Nordic Field Recordings", "sample-pack", "1.0.0",
     "Ice, wind, forests and harbours recorded across Norway and Finland.", CC0,
     {"genres": {"primary": "ambient", "secondary": ["cinematic"]}, "instruments": ["fx"], "moods": ["dreamy", "dark"], "useCases": ["sound-design"], "energy": 0.15},
     None,
     [("ambiences/harbour-wind.wav", "Harbour Wind", "one-shot", "fx", "fx", 4.0), ("ambiences/ice-creak.wav", "Ice Creak", "one-shot", "fx", "fx", 2.5)],
     {"tags": {"free": ["field-recording", "nature", "texture"]}}),
    ("acme", "industrial-techno", "Warehouse Techno", "sample-pack", "3.1.0",
     "Industrial percussion, distorted kicks and metallic textures captured in an empty warehouse.", CC_BY,
     {"genres": {"primary": "industrial-techno", "secondary": ["techno"]}, "instruments": ["drums", "kick", "percussion"], "moods": ["dark", "aggressive"], "energy": 0.88},
     {"bpm": {"value": 132, "range": [128, 136]}, "key": {"tonic": "D", "mode": "minor"}},
     [("loops/metal-perc-132.wav", "Metal Perc", "loop", "percussion", "drums", 1.8), ("one-shots/distorted-kick.wav", "Distorted Kick", "one-shot", "kick", "drums", 0.5)],
     {"tags": {"free": ["industrial", "metallic", "distorted"]}}),
    ("lattice", "plate-verb", "Plate Verb", "effect", "2.3.0",
     "Algorithmic plate reverb modelled after classic studio plates.", MIT,
     {"genres": {"primary": "electronic", "secondary": []}, "instruments": [], "moods": ["warm"], "useCases": ["mixing"]},
     None, [], {"module": module("audio-effect", ["audio", "automation"], ["audio"], ["insert", "send", "bus"], ["reverb"]),
                "tags": {"free": ["reverb", "plate", "open-source"]}, "technical": {"formats": ["clap"]}}),
    ("lattice", "grain-cloud", "Grain Cloud", "instrument", "0.9.0",
     "Granular synthesizer with 8 grain streams and spectral freeze.", MIT,
     {"genres": {"primary": "ambient", "secondary": ["cinematic"]}, "instruments": ["synth", "pad"], "moods": ["dreamy"], "useCases": ["sound-design"]},
     None, [], {"module": module("instrument", ["midi", "automation"], ["audio"], ["instrument-track"], ["granular", "sampler"]),
                "tags": {"free": ["granular", "texture"]}, "technical": {"formats": ["clap"]},
                "dependencies": [{"id": "fieldnotes.nordic-field-recordings", "range": "^1.0.0"}]}),
    ("lattice", "polyarp", "PolyArp", "midi-tool", "1.2.1",
     "Polyrhythmic arpeggiator with per-step probability and scale quantisation.", MIT,
     {"genres": {"primary": "techno", "secondary": ["melodic-techno"]}, "instruments": [], "moods": ["hypnotic"], "useCases": ["live-performance"]},
     None, [], {"module": module("note-effect", ["midi", "transport"], ["midi"], ["midi-track"], ["arpeggiator"]),
                "tags": {"free": ["arp", "generative"]}, "technical": {"formats": ["clap"]}}),
]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--api", default="http://localhost:5173/api/v1")
    ap.add_argument("--email", default="demo@openaudio.dev")
    ap.add_argument("--password", default="openaudio-demo-password")
    args = ap.parse_args()
    api = API(args.api)
    user = api.sign_in(args.email, args.password, "OpenAudio Demo")
    if user["role"] != "admin":
        sys.exit(f"{args.email} is not an admin. Start the API with ADMIN_EMAILS={args.email} (before the account is created).")

    for slug, name in PUBLISHERS:
        st, res = api.call("POST", "/publishers", {"slug": slug, "name": name})
        print(f"publisher @{slug}: {'created' if st == 201 else res.get('code')}")

    for pub, slug, name, kind, version, desc, lic, tax, musical, samples, extra in PACKAGES:
        api.call("POST", "/packages", {"publisher": pub, "slug": slug, "name": name, "kind": kind, "description": desc})
        artifacts, sample_meta = [], []
        key = (musical or {}).get("key", {}).get("tonic")
        for i, (path, sname, ctype, instrument, role, seconds) in enumerate(samples):
            data = tone_wav(freq_of(key, 2 + i % 3), seconds)
            digest = api.upload(data, "audio/wav")
            artifacts.append({"kind": "sample", "path": path, "digest": digest, "size": len(data), "contentType": "audio/wav"})
            meta = {"id": sname, "file": path, "contentType": ctype, "instrument": instrument, "role": role, "durationSec": seconds}
            if ctype == "loop" and musical and "bpm" in musical:
                meta["bpm"] = musical["bpm"]["value"]
            sample_meta.append(meta)
        if not artifacts:  # module packages ship a binary
            data = (name + " module binary\n").encode() * 64
            digest = api.upload(data, "application/octet-stream")
            artifacts.append({"kind": "binary", "path": f"bin/{slug}.clap", "digest": digest, "size": len(data), "contentType": "application/octet-stream"})
        manifest = {"version": version, "description": desc, "license": lic, "taxonomy": tax, "artifacts": artifacts}
        if musical:
            manifest["musical"] = musical
        if sample_meta:
            manifest["samples"] = sample_meta
            manifest["technical"] = {"sampleRate": 22050, "bitDepth": 16, "formats": ["wav"]}
        manifest.update(extra)
        st, res = api.call("POST", f"/packages/{pub}/{slug}/releases", {"changelog": "Initial release", "manifest": manifest})
        if st == 409:
            print(f"{pub}/{slug}@{version}: already exists")
            continue
        if st != 201:
            sys.exit(f"{pub}/{slug}: draft failed: {json.dumps(res, indent=2)}")
        st, res = api.call("POST", f"/packages/{pub}/{slug}/releases/{version}/publish")
        print(f"{pub}/{slug}@{version}: {'published' if st == 200 else res}")


if __name__ == "__main__":
    main()
