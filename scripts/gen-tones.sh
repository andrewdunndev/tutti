#!/usr/bin/env bash
# Generates the test-tone matrix that tutti embeds and serves during
# drive tests. Reproducible from this script alone; tones are MIT-
# licensed alongside the rest of the repo.
#
# Each tone is a 5-second 1 kHz sine at -6 dBFS, mono. Embedded metadata
# uses the "tutti embed:" prefix so a renderer that reads file tags can
# be distinguished from one that uses DIDL-Lite (see README).

set -uo pipefail

OUT="${1:-internal/audio/tones}"
mkdir -p "$OUT"

DURATION=5
FREQ=1000
GAIN=-6  # dBFS

ARTIST="tutti.dunn.dev"
ALBUM="Audio Renderer Probes (embedded)"

gen() {
  local name="$1"; local rate="$2"; local bits="$3"; local fmt="$4"; local extra="$5"
  local title="tutti embed: ${name}"
  local out="${OUT}/${name// /_}.${fmt}"
  local fade_st
  fade_st=$(echo "$DURATION - 0.05" | bc -l)
  local lavfi="sine=frequency=${FREQ}:sample_rate=${rate}:duration=${DURATION},volume=${GAIN}dB,afade=t=in:d=0.05,afade=t=out:st=${fade_st}:d=0.05"
  if ffmpeg -y -f lavfi -i "$lavfi" \
        -metadata "title=${title}" \
        -metadata "artist=${ARTIST}" \
        -metadata "album=${ALBUM}" \
        ${extra} \
        "$out" 2>/dev/null
  then
    echo "  -> $out"
  else
    echo "  !! skipped (encoder unavailable): $name"
  fi
}

gen "WAV PCM 44k 16"   44100 16 wav  "-c:a pcm_s16le"
gen "WAV PCM 48k 16"   48000 16 wav  "-c:a pcm_s16le"
gen "WAV PCM 96k 24"   96000 24 wav  "-c:a pcm_s24le"
gen "WAV PCM 192k 24" 192000 24 wav  "-c:a pcm_s24le"
gen "FLAC 44k 16"      44100 16 flac "-c:a flac -compression_level 5 -sample_fmt s16"
gen "FLAC 96k 24"      96000 24 flac "-c:a flac -compression_level 5 -sample_fmt s32"
gen "MP3 320"          44100 16 mp3  "-c:a libmp3lame -b:a 320k"
gen "AAC 256"          44100 16 m4a  "-c:a aac -b:a 256k"
gen "Opus 128"         48000 16 opus "-c:a libopus -b:a 128k"

echo
echo "Generated $(ls -1 "$OUT" | wc -l | tr -d ' ') tones in $OUT"
