import re
from typing import Optional


GRADER_PATTERN = re.compile(r"\b(PSA|CGC|BGS|BECKETT|SGC|ACE|TAG)\b", re.IGNORECASE)
CERT_PATTERN = re.compile(r"\b(?:CERT|CERTIFICATE|CERT\s*#|CERTIFICATION)\s*[:#]?\s*(\d{5,})\b", re.IGNORECASE)
GRADE_PATTERNS = [
    re.compile(r"\b(?:GEM\s+MINT|GEM\s+MT)\s*10\b", re.IGNORECASE),
    re.compile(r"\b(?:PRISTINE|PERFECT)\s*10\b", re.IGNORECASE),
    re.compile(r"\b(10|9\.5|9|8\.5|8|7\.5|7|6\.5|6|5\.5|5)\b", re.IGNORECASE),
]


def parse_slab(title: str, condition: Optional[str] = None) -> dict:
    text = " ".join(part for part in [title or "", condition or ""] if part)
    grader_match = GRADER_PATTERN.search(text)
    if not grader_match:
        if re.search(r"\b(graded|slabbed|gem\s+mint|gem\s+mt)\b", text, re.IGNORECASE):
            return {
                "is_slab": True,
                "grader": None,
                "grade": None,
                "slab_tier": "GRADED_UNKNOWN",
                "cert_number": _cert_number(text),
            }
        return {
            "is_slab": False,
            "grader": None,
            "grade": None,
            "slab_tier": "RAW",
            "cert_number": None,
        }

    grader = grader_match.group(1).upper()
    if grader == "BECKETT":
        grader = "BGS"

    grade = _grade(text, grader)
    black_label = bool(re.search(r"\bblack\s+label\b", text, re.IGNORECASE))
    if black_label and grader == "BGS" and grade == "10":
        slab_tier = "BGS_BLACK_LABEL"
    elif grade:
        slab_tier = f"{grader}_{grade.replace('.', '_')}"
    else:
        slab_tier = f"{grader}_UNKNOWN"

    return {
        "is_slab": True,
        "grader": grader,
        "grade": grade,
        "slab_tier": slab_tier,
        "cert_number": _cert_number(text),
    }


def _grade(text: str, grader: str) -> Optional[str]:
    grader_index = text.upper().find(grader)
    window = text[max(0, grader_index): grader_index + 80] if grader_index >= 0 else text
    for pattern in GRADE_PATTERNS:
        match = pattern.search(window)
        if not match:
            continue
        if match.groups():
            return match.group(1)
        return "10"
    return None


def _cert_number(text: str) -> Optional[str]:
    match = CERT_PATTERN.search(text)
    return match.group(1) if match else None
