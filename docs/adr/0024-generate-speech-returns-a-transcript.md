# 0024. `generate_speech` returns a transcript, and the calling agent checks the audio

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the owner, in the planning session of epic #181

## Context

An agent can open a PNG and look at it. An agent cannot listen to an audio
file. The end-to-end test of the test repo `my-tss` checks the file, its
length and its loudness, and it says: a person must listen to prove the words.

The owner does not listen to each file. The words of the owner: "the goal is
to let ai agent work together".

## Decision

- `generate_speech` returns the audio file and its transcript: the words that
  a speech recognition model reads from that file.
- The calling agent compares the transcript with its text and decides. horus
  gives no verdict.
- The speech recognition model is a part of the speech workflow: its files
  are in `models.json`, and `install_workflow` downloads them.
- The owner approves that model before its first download, as for each new
  model.
- No goal row and no task needs a person who listens.

## Consequences

- An agent can find a wrong word, a missing sentence or an empty file without
  a person.
- A speech workflow downloads one more model, and each call takes more time.
- A difference between the text and the transcript has two possible causes:
  the speech model or the speech recognition model. The caller cannot tell
  them apart from one call.
- The transcript proves the words. It does not prove the tone or the quality
  of the voice.
- To reverse this decision, remove the field `transcript` and the model. Then
  a person must listen again.
