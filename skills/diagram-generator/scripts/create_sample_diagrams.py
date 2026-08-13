#!/usr/bin/env python3
"""Create sample diagram sources for smoke testing the diagram-generator skill."""

from pathlib import Path

SAMPLES = {
    "sample_flow.mmd": """flowchart TD\n  start([Start]) --> validate{Valid?}\n  validate -- yes --> done([Done])\n  validate -- no --> fix[Fix input]\n  fix --> validate\n""",
    "sample_graph.dot": """digraph G {\n  rankdir=LR;\n  node [shape=box, style=rounded];\n  app -> api;\n  api -> db;\n}\n""",
    "sample_sequence.puml": """@startuml\nactor User\nparticipant API\nUser -> API: Request\nAPI --> User: Response\n@enduml\n""",
    "sample.svg": """<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"200\" height=\"80\" role=\"img\"><title>Sample</title><rect x=\"10\" y=\"10\" width=\"180\" height=\"60\" fill=\"white\" stroke=\"black\"/><text x=\"100\" y=\"45\" text-anchor=\"middle\">Sample</text></svg>\n""",
}


def main() -> int:
    out_dir = Path.cwd() / "samples"
    out_dir.mkdir(exist_ok=True)
    for name, content in SAMPLES.items():
        path = out_dir / name
        path.write_text(content, encoding="utf-8")
        print(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
