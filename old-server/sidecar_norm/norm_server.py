# norm_server.py
from fastapi import FastAPI
from pydantic import BaseModel

try:
    from fastapi.responses import ORJSONResponse as JSONResp
except Exception:
    from fastapi.responses import JSONResponse as JSONResp # fallback

# ╰─➤  uvicorn norm_server:app --uds /tmp/norm.sock --http httptools --loop uvloop 
# chmod 666 /tmp/norm.sock

# ╰─➤  curl --unix-socket /tmp/norm.sock http://unix/healthz 
    
# 47-language ONNX model (punctuation+truecasing+segmentation) 
# https://huggingface.co/1-800-BAD-CODE/punct_cap_seg_47_language?utm_source=chatgpt.com

from typing import List, Union
# Def helpers to catch ITN edge cases

import re
import os

USE_PCS = os.getenv("USE_PCS", "0") not in ("0","false","False","")
USE_ITN = os.getenv("USE_ITN", "0") not in ("0", "false", "False", "")

# ---- optional NeMo ITN ----
_ITN_READY = False
itn = None
if USE_ITN:
    try:
        from nemo_text_processing.inverse_text_normalization.inverse_normalize import InverseNormalizer
        itn = InverseNormalizer(lang="en")
        _ITN_READY = True
    except Exception as e:
        print(f"[norm] NeMo ITN disabled: {e}", flush=True)
        _ITN_READY = False
        itn = None
        
_pcs_err = None
_PCS_READY = False
pcs_model = None
if USE_PCS:
    try: 
        from punctuators.models import PunctCapSegModelONNX
        pcs_model = PunctCapSegModelONNX.from_pretrained(
            "pcs_47lang",
            ort_providers=["CPUExecutionProvider"]
        )
        _PCS_READY = True
    except Exception as e:
        print(f"[norm] PCS model disabled: {e}", flush=True)
        _pcs_err = str(e)
        pcs_model = None

   

# Reuse your word sets:
ONES_MAP = {
    "zero":0,"oh":0,"o":0,"one":1,"two":2,"three":3,"four":4,"five":5,
    "six":6,"seven":7,"eight":8,"nine":9
}
TEENS_MAP = {
    "ten":10,"eleven":11,"twelve":12,"thirteen":13,"fourteen":14,
    "fifteen":15,"sixteen":16,"seventeen":17,"eighteen":18,"nineteen":19
}
TENS_MAP = {
    "twenty":20,"thirty":30,"forty":40,"fifty":50,"sixty":60,"seventy":70,"eighty":80,"ninety":90,
}


YEAR_ALLOW = set(ONES_MAP) | set(TEENS_MAP) | set(TENS_MAP) | {"hundred","thousand","and"}
WORD_TOKEN = re.compile(r"\b([A-Za-z]+)\b")

def _parse_two_digit_str(tokens: list[str]) -> tuple[str|None, int]:
    """Return ('02'-'99', consumed) for small numbers in year tails."""
    if not tokens: return None, 0
    t0 = tokens[0].lower()
    # 'oh three' / 'o three' / 'zero three'
    if t0 in ("oh","o","zero"):
        if len(tokens) >= 2 and tokens[1].lower() in ONES_MAP:
            return f"0{ONES_MAP[tokens[1].lower()]}", 2
        return "00", 1
    if t0 in TEENS_MAP:
        return f"{TEENS_MAP[t0]:02d}", 1
    if t0 in TENS_MAP:
        val, c = TENS_MAP[t0], 1
        if len(tokens) >= 2 and tokens[1].lower() in ONES_MAP:
            val += ONES_MAP[tokens[1].lower()]
            c = 2
        return f"{val:02d}", c
    if t0 in ONES_MAP:
        return f"{ONES_MAP[t0]:02d}", 1
    return None, 0

def _parse_year_words(tokens: list[str]) -> tuple[int|None, int]:
    """Parse 'two thousand (and) three' | 'nineteen ninety nine' | 'twenty twenty four'."""
    if not tokens: return None, 0
    t0 = tokens[0].lower()

    # two thousand (and) X
    if t0 == "two" and len(tokens) >= 2 and tokens[1].lower() == "thousand":
        idx = 2
        if idx < len(tokens) and tokens[idx].lower() == "and":
            idx += 1
        if idx >= len(tokens):
            return 2000, idx
        tail, c = _parse_two_digit_str(tokens[idx:])
        if tail is None:
            return 2000, idx
        return 2000 + int(tail), idx + c

    # nineteen YY / twenty YY
    if t0 in ("nineteen","twenty"):
        base = 1900 if t0 == "nineteen" else 2000
        yy, c = _parse_two_digit_str(tokens[1:])
        if yy is None:
            return None, 0
        return base + int(yy), 1 + c

    # fallback: simple cardinal (rarely used for full years)
    return None, 0

def _parse_small_cardinal(tokens: list[str]) -> tuple[int|None, int]:
    """Parse small cardinals used for day(1-31)/month(1-12). Supports 'twenty three', 'oh three'."""
    # Try two-digit parser first (handles oh/o/zero …)
    s, c = _parse_two_digit_str(tokens)
    if s is not None:
        return int(s), c
    # very small numbers like 'ten' as day/month
    t0 = tokens[0].lower()
    if t0 in TEENS_MAP: return TEENS_MAP[t0], 1
    return None, 0

def _spoken_numeric_dates_to_iso_prepcs(text: str) -> str:
    """
    Find triples of number-words -> D M Y and replace with YYYY-MM-DD.
    Keeps everything else (measurements etc.) intact by requiring 3 groups.
    """
    # Tokenize whole string to words with indices
    words = list(WORD_TOKEN.finditer(text))
    if not words:
        return text

    i = 0
    out = []
    last = 0

    def valid_day(v: int) -> bool: return 1 <= v <= 31
    def valid_mon(v: int) -> bool: return 1 <= v <= 12
    def valid_year(v: int) -> bool: return 1900 <= v <= 2099

    while i < len(words):
        # Try to read 3 consecutive number-word groups
        for j in range(i, min(i+7, len(words))):  # small lookahead
            pass
        # Group 1
        g1 = []
        k = i
        while k < len(words) and words[k].group(1).lower() in YEAR_ALLOW:
            g1.append(words[k].group(1))
            k += 1
        if not g1:
            i += 1
            continue
        # Require a space between groups (i.e., next word exists and next group starts)
        # Group 2
        g2 = []
        k2 = k
        while k2 < len(words) and words[k2].group(1).lower() in YEAR_ALLOW:
            g2.append(words[k2].group(1))
            k2 += 1
        if not g2:
            i += 1
            continue
        # Group 3
        g3 = []
        k3 = k2
        while k3 < len(words) and words[k3].group(1).lower() in YEAR_ALLOW:
            g3.append(words[k3].group(1))
            k3 += 1
        if not g3:
            i += 1
            continue

        # Interpret as D M Y
        d, c1 = _parse_small_cardinal(g1)
        m, c2 = _parse_small_cardinal(g2)
        y, c3 = _parse_year_words(g3)

        if d is None or m is None or y is None or not (valid_day(d) and valid_mon(m) and valid_year(y)):
            i += 1
            continue

        # Replace span [start of g1 .. end of g3]
        span_start = words[i].start()
        span_end = words[k3-1].end()
        out.append(text[last:span_start])
        out.append(f"{y:04d}-{m:02d}-{d:02d}")
        last = span_end
        i = k3
        continue

    out.append(text[last:])
    return "".join(out)


# ----- numeric cleanup before ITN (fixed) -----
_NUM_WORD = (
    r"(?:zero|oh|one|two|three|four|five|six|seven|eight|nine|"
    r"ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|"
    r"twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|hundred|thousand)"
)
_NUM_LIKE = rf"(?:\d+|{_NUM_WORD})"

_re_lower_oh_between_nums = re.compile(rf"\b({_NUM_LIKE})\b\s+Oh\s+\b({_NUM_LIKE})\b")
_re_strip_commas_between_nums = re.compile(rf"\b({_NUM_LIKE})\b\s*,\s*\b({_NUM_LIKE})\b")
_re_oh_to_zero_when_followed = re.compile(rf"\boh\b(\s+{_NUM_LIKE}\b)", re.IGNORECASE)


def _pre_itn_cleanup(text: str) -> str:
    t = _re_lower_oh_between_nums.sub(r"\1 oh \2", text)
    t = _re_strip_commas_between_nums.sub(r"\1 \2", t)
    t = _re_oh_to_zero_when_followed.sub(r"zero\1", t)
    return t

# ----- ISO date normalization after ITN -----
_MONTH_MAP = {
    "january": 1, "jan": 1, "february": 2, "feb": 2, "march": 3, "mar": 3,
    "april": 4, "apr": 4, "may": 5, "june": 6, "jun": 6, "july": 7, "jul": 7,
    "august": 8, "aug": 8, "september": 9, "sep": 9, "sept": 9,
    "october": 10, "oct": 10, "november": 11, "nov": 11, "december": 12, "dec": 12,
}
_MONTH_WORDS = r"(?:jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:t(?:ember)?)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)"

# DMY with month WORD: "11th of March, 2024" | "11 March 2024"
_re_dmy_text = re.compile(rf"\b(?:the\s+)?(\d{{1,2}})(?:st|nd|rd|th)?\s*(?:of\s+)?({_MONTH_WORDS})\s*,?\s*((?:19|20)\d{{2}})\b", re.IGNORECASE)

def _to_iso(y: int, m: int, d: int) -> str:
    return f"{y:04d}-{m:02d}-{d:02d}"

def _repl_dmy_text(m: re.Match) -> str:
    d = int(m.group(1))
    mon = _MONTH_MAP[m.group(2).lower()]
    y = int(m.group(3))
    return _to_iso(y, mon, d)


def _dates_to_iso(text: str) -> str:
    """Normalize only dates with month *words* (e.g., '11th of March 2024').
       Leave numeric forms like 11/04/2024 untouched.
    """
    return _re_dmy_text.sub(_repl_dmy_text, text)

# Optional: tidy spaces around punctuation (nice-to-have)
_re_space_before_punct = re.compile(r"\s+([,.;:!?])")
_re_multi_space = re.compile(r"[ \t]{2,}")

def _tidy_punct(text: str) -> str:
    t = _re_space_before_punct.sub(r"\1", text)
    t = _re_multi_space.sub(" ", t)
    t = _re_multi_dots.sub(".", t)   # collapse repeated dots
    t = re.sub(r"\s+\.", ".", t)
    t = re.sub(r"\s+,", ",", t)
    return t.strip()


# ----- AU phone canonicalization after ITN -----
_WORD2DIGIT = {
    "zero": "0", "oh": "0", "o": "0",
    "one": "1", "two": "2", "three": "3", "four": "4", "five": "5",
    "six": "6", "seven": "7", "eight": "8", "nine": "9",
    # (rare in phone dictation, but handle anyway)
    "ten": "10", "eleven": "11", "twelve": "12", "thirteen": "13", "fourteen": "14",
    "fifteen": "15", "sixteen": "16", "seventeen": "17", "eighteen": "18", "nineteen": "19",
}

# Find long runs of number-like tokens (7+), which is typical of phone sequences
_MIN_PHONE_TOKENS = 5
_re_phone_candidate = re.compile(
    rf"((?:\b(?:{_NUM_WORD}|\d+|plus|\+|double|triple)\b[\s,;/\-]*){{{_MIN_PHONE_TOKENS},}})",
    re.IGNORECASE,
)

# Tokenizer for the candidate chunk
_re_phone_token = re.compile(rf"(?:{_NUM_WORD}|\d+|plus|\+|double|triple)", re.IGNORECASE)

def _format_au_number(digits: str, plus: bool) -> str | None:
    # +61 mobile
    if plus and digits.startswith("61") and len(digits) == 11 and digits[2] == "4":
        return f"+61 {digits[2:6]} {digits[6:9]} {digits[9:]}"
    # +61 landline
    if plus and digits.startswith("61") and len(digits) == 11 and digits[2] in "2378":
        return f"+61 {digits[2]} {digits[3:7]} {digits[7:]}"
    # local mobile 04xx xxx xxx
    if digits.startswith("04") and len(digits) == 10:
        return f"{digits[:4]} {digits[4:7]} {digits[7:]}"
    # local landline 0A xxxx xxxx
    if digits.startswith("0") and len(digits) == 10 and digits[1] in "2378":
        return f"{digits[:2]} {digits[2:6]} {digits[6:]}"
    # fallback: if plausible length, just emit digits with optional + (safer than leaving commas)
    if 8 <= len(digits) <= 12:
        return f"+{digits}" if plus and not digits.startswith("61") else digits
    return None

def _canon_phone_chunk(chunk: str) -> str | None:
    tokens = _re_phone_token.findall(chunk)
    if not tokens:
        return None

    digits = []
    plus = False
    i = 0
    while i < len(tokens):
        tok = tokens[i].lower()
        if tok in ("+", "plus"):
            plus = True
            i += 1
            continue
        if tok in ("double", "triple") and i + 1 < len(tokens):
            nxt = tokens[i + 1].lower()
            d = _WORD2DIGIT.get(nxt)
            if d is None and re.fullmatch(r"\d", nxt):
                d = nxt
            if d:
                rep = d * (2 if tok == "double" else 3)
                digits.append(rep)
                i += 2
                continue
        # single number word or digits
        d = _WORD2DIGIT.get(tok)
        if d is not None:
            digits.append(d)
            i += 1
            continue
        if re.fullmatch(r"\d+", tok):
            digits.append(tok)
            i += 1
            continue
        i += 1

    joined = "".join(digits)
    if not joined:
        return None
    fmt = _format_au_number(joined, plus)
    return fmt

def _phones_to_au_canonical(text: str) -> str:
    def repl(m: re.Match) -> str:
        chunk = m.group(0)
        canon = _canon_phone_chunk(chunk)
        return canon if canon else chunk
    return _re_phone_candidate.sub(repl, text)


# Force lowercase before PCS to match the 47-lang SP vocab
FORCE_LOWERCASE_FOR_PCS = True

# Safety: strip any stray <unk> tokens (should be unnecessary once we lowercase)
_re_unk_any = re.compile(r"<\s*unk\s*>", re.IGNORECASE)
def _strip_unk(s: str) -> str:
    return _re_unk_any.sub("", s)

# Also collapse "..", "...", etc. during tidy
_re_multi_dots = re.compile(r"\.{2,}")


class Req(BaseModel):
    text: str
    day_first: bool = True # AU bias hint (customize grammars later if needed)
    
class BatchReq(BaseModel):
    texts: list[str]
    day_first: bool = True
    
def _pcs(texts: List[str], apply_sbd: bool = False, overlap: int = 16) -> List[str]:
    """
    Run punctuation+truecasing (and optionally sentence segmentation).
    
    .infer() returns Union[List[str]m List[List[str]]].
    
    Default to apply_sbd=False to get a single string per input.
    """
    
    out: Union[List[str], List[List[str]]] = pcs_model.infer(
        texts=texts,
        apply_sbd=apply_sbd, # False => single string output
        batch_size_tokens=4096,
        overlap=overlap,
        num_workers=0, # safer under WSL/Containers
    )
    
    if isinstance(out[0], list):
        # Join sentences back to one string if segmentation was requested
        return [" ".join(segs) for segs in out] # type: ignore[index]

    return out # type: ignore[return-value]

def _normalize(texts: List[str], day_first: bool) -> List[str]:
    out = []
    for t in texts:
        # 0) pre-PCS: normalize spoken numeric D M Y (words only)
        t0 = _spoken_numeric_dates_to_iso_prepcs(t)
        
        if _PCS_READY and pcs_model is not None:
            # 1) lowercase -> PCS
            t1 = t0.lower() if FORCE_LOWERCASE_FOR_PCS else t0
            punct = _pcs([t1], apply_sbd=False, overlap=16)[0]
            punct = _strip_unk(punct)
        else:
            punct = t0 # Trust STT punctuation

        cleaned = _pre_itn_cleanup(punct)
        # 2) pre-ITN small cleanup
        
        if _ITN_READY and itn is not None:
            try:
                cleaned = itn.inverse_normalize(cleaned, verbose=False)
            except Exception:
                pass

        # 4) phone numbers (AU) and ONLY month-word dates
        phoneed = _phones_to_au_canonical(cleaned)
        isoed = _dates_to_iso(phoneed)  # now word-only

        # 5) tidy
        final = _tidy_punct(isoed)
        out.append(final)
    return out

app = FastAPI(default_response_class=JSONResp)

@app.get("/healthz")
def healthz():
    langs = []
    if pcs_model is not None:
        try:
            langs = pcs_model.languages
        except Exception:
            langs = []
    return {
        "ok": True,
        "pcs_enabled": bool(USE_PCS and pcs_model is not None),
        "pcs_error": _pcs_err,
        "itn_enabled": bool(USE_ITN and _ITN_READY),
        "langs": langs,
    }

@app.post("/norm")
def norm(req: Req):
    return {"text": _normalize([req.text], req.day_first)[0]}

@app.post("/norm_batch")
def norm_batch(req: BatchReq):
    return {"texts": _normalize(req.texts, req.day_first)}