#!/usr/bin/env python3
"""
PDF to Markdown Converter
Converts all PDF files in docs/nfse-nacional/ to markdown format.
Uses PyMuPDF (fitz) for extraction.

Usage:
    pip install PyMuPDF
    python convert_pdfs.py
"""

import fitz  # PyMuPDF
import os
from pathlib import Path


def extract_images(page, pdf_name: str, page_num: int, images_dir: Path) -> list[str]:
    """Extract images from a page and return markdown image references."""
    image_refs = []
    image_list = page.get_images(full=True)

    if not image_list:
        return image_refs

    pdf_images_dir = images_dir / pdf_name
    pdf_images_dir.mkdir(parents=True, exist_ok=True)

    for img_idx, img_info in enumerate(image_list, 1):
        xref = img_info[0]
        try:
            base_image = page.parent.extract_image(xref)
            image_bytes = base_image["image"]
            image_ext = base_image["ext"]

            image_filename = f"page{page_num + 1}_img{img_idx}.{image_ext}"
            image_path = pdf_images_dir / image_filename

            with open(image_path, "wb") as img_file:
                img_file.write(image_bytes)

            relative_path = f"images/{pdf_name}/{image_filename}"
            image_refs.append(f"![Image]({relative_path})")
        except Exception as e:
            print(f"    Warning: Could not extract image {img_idx} from page {page_num + 1}: {e}")

    return image_refs


def convert_pdf_to_markdown(pdf_path: Path, output_dir: Path, images_dir: Path) -> None:
    """Convert a single PDF file to markdown."""
    pdf_name = pdf_path.stem
    print(f"Converting: {pdf_path.name}")

    doc = fitz.open(pdf_path)
    markdown_content = []

    markdown_content.append(f"# {pdf_name.replace('-', ' ').title()}\n")
    markdown_content.append(f"*Converted from: {pdf_path.name}*\n")
    markdown_content.append("---\n")

    for page_num in range(len(doc)):
        page = doc[page_num]

        # Extract text
        text = page.get_text("text")

        if text.strip():
            markdown_content.append(f"\n## Page {page_num + 1}\n")

            # Process text - clean up and format
            lines = text.split('\n')
            processed_lines = []

            for line in lines:
                line = line.strip()
                if line:
                    processed_lines.append(line)

            markdown_content.append('\n'.join(processed_lines))
            markdown_content.append("\n")

        # Extract images
        image_refs = extract_images(page, pdf_name, page_num, images_dir)
        if image_refs:
            markdown_content.append(f"\n### Images from Page {page_num + 1}\n")
            markdown_content.append('\n\n'.join(image_refs))
            markdown_content.append("\n")

    doc.close()

    # Write markdown file
    output_path = output_dir / f"{pdf_name}.md"
    with open(output_path, "w", encoding="utf-8") as f:
        f.write('\n'.join(markdown_content))

    print(f"  -> Created: {output_path.name}")


def main():
    # Define paths
    script_dir = Path(__file__).parent
    pdf_dir = script_dir / "docs" / "nfse-nacional"
    output_dir = script_dir / "docs" / "markdown"
    images_dir = output_dir / "images"

    # Create output directories
    output_dir.mkdir(parents=True, exist_ok=True)
    images_dir.mkdir(parents=True, exist_ok=True)

    # Find all PDF files
    pdf_files = sorted(pdf_dir.glob("*.pdf"))

    if not pdf_files:
        print(f"No PDF files found in {pdf_dir}")
        return

    print(f"Found {len(pdf_files)} PDF files to convert\n")

    # Convert each PDF
    for pdf_path in pdf_files:
        try:
            convert_pdf_to_markdown(pdf_path, output_dir, images_dir)
        except Exception as e:
            print(f"  Error converting {pdf_path.name}: {e}")

    print(f"\nConversion complete!")
    print(f"Markdown files saved to: {output_dir}")
    print(f"Images saved to: {images_dir}")


if __name__ == "__main__":
    main()
