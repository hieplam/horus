---
family: flux3-klein
workflow: workflows/it-illustration
seed: 2024
size: 1536x1024
steps: 15
---

# Message queue between two services

## Caption (house style prefix + subject + house style suffix)

```text
A clean flat vector illustration for a modern tech explainer. It looks like a tidy technical diagram, with dark navy outlines of uniform medium weight and rounded corners, and solid flat pastel fills. On the left, a mint-green server box with a small round icon sends small sun-yellow envelopes onto a long lilac conveyor belt that crosses the middle of the picture. The belt is a message queue, and five envelopes wait in a row on it. On the right, a sky-blue server box takes the envelopes off the end of the belt one by one. Thin navy arrows show the direction from left to right. The colour palette is dark navy for outlines, white, sky blue, lilac, mint green, sun yellow and coral, each with a light pastel tint for fills. Each object has one very soft flat shadow below it. The background is plain very pale blue with a faint sparse dotted grid and nothing else. Shapes are simple and friendly, with generous empty space. The whole scene is centred and fully inside the frame, with wide margins on all sides. Every surface is blank: screens and panels show only abstract coloured bars and blocks. The picture contains no text of any kind: no letters, no digits, no symbols and no logos.
```

## Result

- File: `workflows/it-illustration/examples/message-queue.png`
- Run: `python scripts/run-illustration.py --subject "<the subject sentences above>" --seed 2024 --out workflows/it-illustration/examples/message-queue.png`
- Time: 469.9 s on a GTX 1660 Ti (6 GB). The MCP run of another subject took 217.2 s on the same machine. The cause of the slower run is not verified.
- Check: flat vector, on subject, no visible text.
