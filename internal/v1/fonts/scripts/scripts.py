import os
import sys
from pathlib import Path


def _generate_kern(abs_path: str):
    path = Path(abs_path)

    if path.suffix.lower() != ".afm":
        raise ValueError("file format is not afm")

    kern_path = path.with_suffix(".kern")

    first = True
    kern_data = []

    with open(path, "r", encoding="utf-8", errors="ignore") as f:
        for raw_line in f:
            line = raw_line.strip()

            if not line:
                continue

            if first:
                if not line.startswith("StartFontMetrics"):
                    raise ValueError("file format is not afm")
                first = False

            print(f"line: {line}")

            if line.startswith("KPX"):
                parts = line.split()

                if len(parts) != 4:
                    continue

                _, char1, char2, kern = parts

                if len(char1) != 1 or len(char2) != 1 or len(kern) < 1:
                    continue

                kern_data.append(f"{char1}, {char2}, {kern},")

    with open(kern_path, "w", encoding="utf-8") as out:
        out.write("\n".join(kern_data))


def generate_kern(font_path: str):
    cwd = Path.cwd()

    if font_path == "--all":
        s14dir = cwd / "standard14"

        for path in s14dir.rglob("*"):
            if path.is_file():
                try:
                    _generate_kern(str(path))
                except Exception as e:
                    print(f"failed for {path}: {e}")

    else:
        font_path = font_path.strip()
        abs_path = Path(font_path)

        if not abs_path.is_absolute():
            abs_path = cwd / abs_path

        _generate_kern(str(abs_path))


def main():
    if len(sys.argv) < 2:
        print("scripts require an argument, please read the docs")
        sys.exit(1)

    match sys.argv[1]:
        case "afm-kern":
            if len(sys.argv) < 3:
                print(
                    "afm-kern requires font path. correct usage: afm-kern <path-to-afm>"
                )
                sys.exit(1)

            try:
                generate_kern(sys.argv[2])
            except Exception as e:
                print(e)
                sys.exit(1)

        case _:
            print("please enter a correct argument.")
            sys.exit(1)


if __name__ == "__main__":
    main()