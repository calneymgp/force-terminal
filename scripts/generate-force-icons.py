#!/usr/bin/env python3
"""Render the approved Force Terminal vectors into desktop icon formats."""

from io import BytesIO
from pathlib import Path

import cairosvg
from PIL import Image


ROOT = Path(__file__).resolve().parent.parent
BRAND = ROOT / "assets" / "force-terminal" / "brand"
SIZES = (16, 24, 32, 48, 64, 128, 256, 512, 1024)


def render_icon(size: int) -> Image.Image:
    source = BRAND / ("icon-compact.svg" if size < 256 else "icon.svg")
    data = cairosvg.svg2png(url=str(source), output_width=size, output_height=size)
    return Image.open(BytesIO(data)).convert("RGBA")


def save_png(image: Image.Image, relative_path: str) -> None:
    target = ROOT / relative_path
    target.parent.mkdir(parents=True, exist_ok=True)
    image.save(target, format="PNG", optimize=True)


def main() -> None:
    images = {size: render_icon(size) for size in SIZES}
    for size, image in images.items():
        save_png(image, f"build/icons/{size}x{size}.png")

    ico_sizes = (16, 24, 32, 48, 64, 128, 256)
    images[256].save(
        ROOT / "build" / "icon.ico",
        format="ICO",
        sizes=[(size, size) for size in ico_sizes],
        append_images=[images[size] for size in ico_sizes if size != 256],
    )
    images[1024].save(
        ROOT / "build" / "icon.icns",
        format="ICNS",
        append_images=[images[size] for size in (16, 32, 64, 128, 256, 512)],
    )
    save_png(images[256], "public/logos/force-terminal-icon.png")
    save_png(images[512], "assets/force-terminal/brand/icon.png")

    # Compatibility aliases used by the widget scaffold and preview fixtures.
    for name in ("wave-logo.png", "wave-logo-dark.png", "wave-logo-256.png"):
        save_png(images[256], f"public/logos/{name}")
    save_png(images[256], "tsunami/frontend/public/wave-logo-256.png")
    print("Force Terminal: PNG icons 16–1024px, ICO, ICNS and runtime aliases generated.")


if __name__ == "__main__":
    main()
