import re, pathlib, sys

SRC = pathlib.Path(sys.argv[1])
DST = pathlib.Path(sys.argv[2])

# unDraw ships one flat palette. Each hex becomes a CSS custom property so the
# illustration re-inks itself from the active theme instead of staying violet
# on a paper background. Skin tones are left alone: they read on both stocks.
MAP = {
    '#6c63ff': 'var(--ud-accent)',
    '#ff6584': 'var(--ud-accent2)',
    '#090814': 'var(--ud-ink)',
    '#2f2e41': 'var(--ud-ink)',
    '#3f3d56': 'var(--ud-ink2)',
    '#e6e6e6': 'var(--ud-mid)',
    '#e4e4e4': 'var(--ud-mid)',
    '#e5e5e5': 'var(--ud-mid)',
    '#cccccc': 'var(--ud-mid)',
    '#ccc':    'var(--ud-mid)',
    '#a8a8a8': 'var(--ud-mid)',
    '#f2f2f2': 'var(--ud-soft)',
    '#f1f1f1': 'var(--ud-soft)',
    '#ffffff': 'var(--ud-paper)',
    '#fff':    'var(--ud-paper)',
}

HEX = re.compile(r'#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b')

def sub(m):
    return MAP.get(m.group(0).lower(), m.group(0))

DST.mkdir(parents=True, exist_ok=True)
for f in sorted(SRC.glob('*.svg')):
    s = f.read_text(encoding='utf-8')
    if '<script' in s.lower() or 'onload=' in s.lower():
        raise SystemExit(f'refusing {f.name}: embedded script')
    s = HEX.sub(sub, s)
    # Drop the intrinsic size so the box decides, and the generated layer ids so
    # two illustrations on one page cannot collide.
    s = re.sub(r'\s(?:width|height)="[\d.]+"', '', s, count=2)
    s = re.sub(r'\sid="[0-9a-f-]{8,}"', '', s)
    s = re.sub(r'\sdata-name="[^"]*"', '', s)
    s = s.replace('<svg ', '<svg preserveAspectRatio="xMidYMid meet" ', 1)
    DST.joinpath(f.name).write_text(s.strip() + '\n', encoding='utf-8')
    print('wrote', f.name, len(s))
