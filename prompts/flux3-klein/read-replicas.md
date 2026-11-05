---
family: flux3-klein
workflow: workflows/it-illustration
seed: 1001
size: 1536x1024
steps: 15
---

# A database with read replicas

## Caption (house style prefix + subject + house style suffix)

```text
A clean flat vector illustration for a modern tech explainer. It looks like a tidy technical diagram, with dark navy outlines of uniform medium weight and rounded corners, and solid flat pastel fills. In the centre, a large mint-green database cylinder with a small round icon. Thin navy arrows run from it to three smaller sky-blue copies of the cylinder in a row below it. On the right, a lilac application window shows only abstract coloured bars. Thin sky-blue arrows run from the window to the three small cylinders, and one sun-yellow arrow runs from the window to the large cylinder. The colour palette is dark navy for outlines, white, sky blue, lilac, mint green, sun yellow and coral, each with a light pastel tint for fills. Each object has one very soft flat shadow below it. The background is plain very pale blue with a faint sparse dotted grid and nothing else. Shapes are simple and friendly, with generous empty space. The whole scene is centred and fully inside the frame, with wide margins on all sides. Every surface is blank: screens and panels show only abstract coloured bars and blocks. The picture contains no text of any kind: no letters, no digits, no symbols and no logos.
```

## Result

- File: `workflows/it-illustration/examples/read-replicas.png`
- Run: `python scripts/run-illustration.py --subject "<the subject sentences above>" --seed 1001 --out workflows/it-illustration/examples/read-replicas.png`
- Time: 262.4 s on a GTX 1660 Ti (6 GB).
- Check: flat vector, on subject, no visible text.

## Notes

The first try said "and these are the read replicas" in the subject. The model then drew the words "read replicas" under the cylinders. The subject now describes only what is visible. The sun-yellow write arrow is drawn between the window and the large cylinder, but the sky-blue arrows do not all point at the three small cylinders (see the image).
