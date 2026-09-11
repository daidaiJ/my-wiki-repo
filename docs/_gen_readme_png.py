"""Generate README diagrams as high-DPI PNGs. Run from repo root."""

from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).resolve().parent
FONTS = Path(r"C:\Windows\Fonts")

BG = (13, 17, 23, 255)
SURFACE = (22, 27, 34, 255)
SURFACE_TEAL = (16, 36, 32, 255)
SURFACE_GREEN = (18, 32, 24, 255)
SURFACE_AMBER = (36, 28, 20, 255)
SURFACE_PURPLE = (28, 23, 48, 255)
SURFACE_SKY = (16, 28, 40, 255)
HEAD_TEAL = (23, 51, 40, 255)
HEAD_AMBER = (31, 42, 28, 255)
HEAD_GRAY = (28, 34, 43, 255)
BORDER = (48, 54, 61, 255)
BORDER_SOFT = (38, 44, 52, 255)
BORDER_TEAL = (45, 107, 82, 255)
BORDER_GREEN = (61, 107, 58, 255)
BORDER_AMBER = (107, 83, 64, 255)
BORDER_PURPLE = (76, 61, 114, 255)
BORDER_SKY = (45, 74, 98, 255)
PILL_BG = (30, 51, 72, 255)
PILL_BD = (61, 90, 117, 255)
PILL_FG = (201, 215, 230, 255)
INNER = (15, 24, 34, 255)
INNER_G = (16, 24, 15, 255)
TEXT = (230, 237, 243, 255)
MUTED = (139, 148, 158, 255)
DIM = (110, 118, 129, 255)
TEAL = (94, 234, 212, 255)
SKY = (125, 211, 252, 255)
AMBER = (245, 208, 169, 255)
PURPLE = (196, 181, 253, 255)
GREEN = (134, 239, 172, 255)
ROSE = (253, 164, 175, 255)


def font(name: str, size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(str(FONTS / name), size)


def UI(s: int) -> ImageFont.FreeTypeFont:
    return font("Deng.ttf", s)


def BD(s: int) -> ImageFont.FreeTypeFont:
    return font("Dengb.ttf", s)


def CODE(s: int) -> ImageFont.FreeTypeFont:
    return font("consola.ttf", s)


def new_canvas(w: int, h: int, radius: int = 40) -> tuple[Image.Image, ImageDraw.ImageDraw]:
    img = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    draw.rounded_rectangle((0, 0, w - 1, h - 1), radius=radius, fill=BG)
    sheen = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    ImageDraw.Draw(sheen).rounded_rectangle(
        (0, 0, w - 1, int(h * 0.42)), radius=radius, fill=(255, 255, 255, 8)
    )
    img = Image.alpha_composite(img, sheen)
    return img, ImageDraw.Draw(img)


def rrect(draw, box, r, fill=None, outline=None, width=2):
    draw.rounded_rectangle(box, radius=r, fill=fill, outline=outline, width=width)


def text_h(font_obj) -> int:
    bbox = font_obj.getbbox("Ag中")
    return bbox[3] - bbox[1]


def draw_text(draw, xy, text, font_obj, fill):
    draw.text(xy, text, font=font_obj, fill=fill)


def triangle(draw, tip, direction, size, fill):
    x, y = tip
    s = size
    if direction == "down":
        pts = [(x - s, y - s * 1.35), (x + s, y - s * 1.35), (x, y)]
    elif direction == "right":
        pts = [(x - s * 1.35, y - s), (x - s * 1.35, y + s), (x, y)]
    else:
        pts = [(x + s * 1.35, y - s), (x + s * 1.35, y + s), (x, y)]
    draw.polygon(pts, fill=fill)


def vline_arrow(draw, x, y1, y2, color, width=4, head=10):
    draw.line((x, y1, x, y2 - head), fill=color, width=width)
    triangle(draw, (x, y2), "down", head * 0.75, color)


def hline_arrow(draw, x1, y, x2, color, width=4, head=10):
    draw.line((x1, y, x2 - head, y), fill=color, width=width)
    triangle(draw, (x2, y), "right", head * 0.75, color)


def save(img: Image.Image, name: str):
    path = OUT / name
    rgb = Image.new("RGB", img.size, (13, 17, 23))
    rgb.paste(img, mask=img.split()[-1])
    rgb.save(path, "PNG", optimize=True, compress_level=9)
    print(f"wrote {path.name} {rgb.size} {path.stat().st_size // 1024}KB")


def section_label(draw, xy, label, color):
    draw_text(draw, xy, label, BD(26), color)


def draw_architecture():
    W, H = 2000, 1680
    img, d = new_canvas(W, H)
    pad, gap = 48, 24
    col_w = (W - pad * 2 - gap * 2) // 3

    y = 40
    section_label(d, (pad, y), "01  接入层  ·  Agents", TEAL)

    y = 84
    card_h = 186
    agents = [
        ("Claude Code", TEAL, "SessionStart  →  prepare", "SessionEnd  →  check"),
        ("ZCode", SKY, "Start  →  prepare", "Stop  →  check"),
        ("Qwen / Codex", AMBER, "wiki inject 注入规约", "开工 prepare  ·  收工 check"),
    ]
    for i, (title, accent, l1, l2) in enumerate(agents):
        x = pad + i * (col_w + gap)
        rrect(d, (x, y, x + col_w, y + card_h), 22, fill=SURFACE, outline=BORDER, width=2)
        d.ellipse((x + 28, y + 34, x + 52, y + 58), fill=accent)
        draw_text(d, (x + 68, y + 28), title, BD(34), TEXT)
        draw_text(d, (x + 28, y + 88), l1, UI(26), MUTED)
        draw_text(d, (x + 28, y + 128), l2, UI(26), MUTED)
        vline_arrow(d, x + col_w // 2, y + card_h + 6, y + card_h + 44, accent)

    y = y + card_h + 62
    section_label(d, (pad, y), "02  控制面  ·  wiki CLI", SKY)
    y += 44
    cli_h = 228
    rrect(d, (pad, y, W - pad, y + cli_h), 22, fill=SURFACE_SKY, outline=BORDER_SKY, width=2)
    draw_text(d, (pad + 32, y + 22), "wiki", BD(36), SKY)
    draw_text(d, (pad + 128, y + 30), "单二进制  ·  stdout 恒空  ·  失败不阻塞会话", UI(26), MUTED)

    cmds = ["prepare", "check", "init", "grep", "cat", "blog", "inject", "bundle"]
    px, py = pad + 32, y + 90
    for cmd in cmds:
        tw = d.textlength(cmd, font=CODE(24))
        pw, ph = int(tw) + 40, 48
        rrect(d, (px, py, px + pw, py + ph), 14, fill=PILL_BG, outline=PILL_BD, width=2)
        draw_text(d, (px + 20, py + 10), cmd, CODE(24), PILL_FG)
        px += pw + 12

    pkgs = [
        ("registry", TEAL, "接入 / 窗口 / 同步"),
        ("view", SKY, "跨项目检索"),
        ("blog", AMBER, "Hugo 流水线"),
        ("config", PURPLE, "WIKI_ROOT"),
        ("guide", ROSE, "规约注入"),
    ]
    px, py = pad + 32, y + 164
    draw_text(d, (px, py), "包", UI(26), DIM)
    px += 52
    for name, color, desc in pkgs:
        draw_text(d, (px, py), name, CODE(24), color)
        px += d.textlength(name, font=CODE(24)) + 10
        draw_text(d, (px, py), desc, UI(26), DIM)
        px += d.textlength(desc, font=UI(26)) + 32

    vline_arrow(d, pad + col_w // 2, y + cli_h + 6, y + cli_h + 44, TEAL)
    vline_arrow(d, W - pad - col_w // 2, y + cli_h + 6, y + cli_h + 44, AMBER)

    y = y + cli_h + 62
    section_label(d, (pad, y), "03  数据面  ·  反转存储", AMBER)
    y += 44
    data_h = 380
    left_w = (W - pad * 2 - gap) // 2

    rrect(d, (pad, y, pad + left_w, y + data_h), 22, fill=SURFACE, outline=BORDER, width=2)
    draw_text(d, (pad + 28, y + 24), "项目仓库（窗口）", BD(32), TEXT)
    draw_text(d, (pad + 28, y + 72), "agent 仍写 wiki/note.md，路径不变", UI(26), MUTED)

    inner = [
        ("项目A / wiki/", "link  ·  symlink / junction  →  知识库正本", TEAL),
        ("项目B / wiki/", "copy  ·  真目录归项目 git  ·  mtime 新者胜  ·  永不删", AMBER),
    ]
    iy = y + 124
    for path, note, accent in inner:
        rrect(d, (pad + 24, iy, pad + left_w - 24, iy + 108), 16, fill=INNER, outline=BORDER_SOFT, width=2)
        draw_text(d, (pad + 44, iy + 18), path, BD(26), PILL_FG)
        draw_text(d, (pad + 44, iy + 60), note, UI(24), accent)
        iy += 120

    rx = pad + left_w + gap
    rrect(d, (rx, y, W - pad, y + data_h), 22, fill=SURFACE_GREEN, outline=BORDER_GREEN, width=2)
    draw_text(d, (rx + 28, y + 24), "WIKI_ROOT（可 git / Obsidian vault）", BD(32), TEXT)
    draw_text(d, (rx + 28, y + 72), "环境变量  →  exe 目录  →  cwd  →  exe 兜底", UI(26), MUTED)

    rows = [
        ("index.md", "隐藏 JSON 注册表 + Markdown 视图"),
        ("projects/<项目>/", "知识正文真文件"),
    ]
    iy = y + 124
    for path, note in rows:
        rrect(d, (rx + 24, iy, W - pad - 24, iy + 72), 14, fill=INNER_G, outline=BORDER_GREEN, width=2)
        draw_text(d, (rx + 44, iy + 20), path, BD(26), GREEN)
        nw = d.textlength(note, font=UI(24))
        draw_text(d, (W - pad - 44 - nw, iy + 22), note, UI(24), MUTED)
        iy += 84

    split = (W - pad - (rx + 24) - 24 - 16) // 2
    rrect(d, (rx + 24, iy, rx + 24 + split, iy + 64), 14, fill=INNER_G, outline=BORDER_GREEN, width=2)
    draw_text(d, (rx + 44, iy + 16), "config.json", BD(26), GREEN)
    rrect(d, (rx + 24 + split + 16, iy, W - pad - 24, iy + 64), 14, fill=INNER_G, outline=BORDER_GREEN, width=2)
    draw_text(d, (rx + 24 + split + 36, iy + 16), "blog.json", BD(26), GREEN)

    mid_y = y + 190
    hline_arrow(d, pad + left_w - 2, mid_y, rx + 4, AMBER, width=4, head=12)
    tw = d.textlength("写", font=UI(22))
    draw_text(d, (pad + left_w + gap / 2 - tw / 2, mid_y - 38), "写", UI(22), AMBER)

    y = y + data_h + 36
    section_label(d, (pad, y), "04  两个出口", PURPLE)
    y += 42
    exits = [
        (SURFACE_PURPLE, BORDER_PURPLE, PURPLE, "Obsidian 校对", "vault 根 = WIKI_ROOT", "整条链路唯一的人工关卡"),
        (SURFACE_SKY, BORDER_SKY, SKY, "git 同步知识库", "projects/ + index.md → private remote", "换机器 clone 后 prepare 重建窗口"),
        (SURFACE_AMBER, BORDER_AMBER, AMBER, "Hugo 发布 / 按需归档", "blog publish  ·  wiki bundle", "push 失败不重试  ·  bundle 不走 hook"),
    ]
    eh = 168
    for i, (bg, bdcol, accent, title, l1, l2) in enumerate(exits):
        x = pad + i * (col_w + gap)
        rrect(d, (x, y, x + col_w, y + eh), 22, fill=bg, outline=bdcol, width=2)
        draw_text(d, (x + 28, y + 24), title, BD(30), accent)
        draw_text(d, (x + 28, y + 76), l1, UI(24), accent)
        draw_text(d, (x + 28, y + 114), l2, UI(24), MUTED)

    save(img, "architecture.png")


def draw_layouts():
    W, H = 2000, 920
    img, d = new_canvas(W, H)
    pad, gap = 40, 24
    col_w = (W - pad * 2 - gap * 2) // 3
    y, h = 40, H - 80

    cols = [
        {
            "title": "正向聚合",
            "badge": "已退役",
            "bg": SURFACE,
            "border": BORDER,
            "head": HEAD_GRAY,
            "title_c": (148, 163, 184, 255),
            "badge_bg": (42, 49, 64, 255),
            "badge_fg": (148, 163, 184, 255),
            "lead": "知识正文留在各项目仓",
            "paths": [("项目 / wiki/", "真目录"), ("WIKI_ROOT / …", "正向链接 →")],
            "body": ["知识库用符号链接「聚合」各项目。", "git 只能提交链接，换机器即断。", "Obsidian 也只能跟着链接走。"],
            "foot": ["你不用选它。", "init / sync --fix 发现正向链接，", "会把正文迁进知识库，变成反转存储。"],
            "foot0": DIM,
            "path_c": DIM,
            "body_c": MUTED,
        },
        {
            "title": "反转存储",
            "badge": "现行",
            "bg": SURFACE_TEAL,
            "border": BORDER_TEAL,
            "head": HEAD_TEAL,
            "title_c": TEAL,
            "badge_bg": (15, 61, 46, 255),
            "badge_fg": TEAL,
            "lead": "知识正文在知识库真文件",
            "paths": [("projects / <项目> / wiki/", "真文件"), ("项目 / wiki/", "窗口 →")],
            "body": ["git 可同步，Obsidian 直读真文件。", "agent 仍写 项目/wiki/，路径不变。", "prepare / check 维护窗口与注册表。"],
            "foot": ["再选项目侧怎么看见正文：", "link（缺省）窗口链接，一份正本", "copy  真目录 + 增量拷贝，永不删"],
            "foot0": TEAL,
            "path_c": GREEN,
            "body_c": TEXT,
            "extra": "link / copy 不是另一种布局。",
        },
        {
            "title": "按需归档",
            "badge": "备份",
            "bg": SURFACE_AMBER,
            "border": BORDER_AMBER,
            "head": HEAD_AMBER,
            "title_c": AMBER,
            "badge_bg": (61, 46, 28, 255),
            "badge_fg": AMBER,
            "lead": "从知识库正本克隆一份目录树",
            "paths": [("wiki bundle [--archive …]", ""), ("bundle/  或  *.zip / *.tar.gz", "")],
            "body": ["窗口链接在打包时被解析成真文件。", "不改项目侧、不走 hook。", "link / copy 两种模式都能打。"],
            "foot": ["不是第三种日常布局。", "用途：跨机器拷贝、离线备份。", "日常同步请用知识库自己的 git。"],
            "foot0": AMBER,
            "path_c": AMBER,
            "body_c": TEXT,
        },
    ]

    for i, c in enumerate(cols):
        x = pad + i * (col_w + gap)
        rrect(d, (x, y, x + col_w, y + h), 26, fill=c["bg"], outline=c["border"], width=2)
        rrect(d, (x, y, x + col_w, y + 96), 26, fill=c["head"])
        d.rectangle((x, y + 70, x + col_w, y + 96), fill=c["head"])
        draw_text(d, (x + 28, y + 26), c["title"], BD(34), c["title_c"])
        tw = d.textlength(c["badge"], font=UI(22))
        bx = x + col_w - 28 - tw - 28
        rrect(d, (bx, y + 30, bx + tw + 28, y + 66), 10, fill=c["badge_bg"])
        draw_text(d, (bx + 14, y + 34), c["badge"], UI(22), c["badge_fg"])

        draw_text(d, (x + 28, y + 120), c["lead"], UI(26), TEXT)
        py = y + 168
        for path, tag in c["paths"]:
            rrect(d, (x + 24, py, x + col_w - 24, py + 52), 12, fill=(12, 16, 22, 80), outline=None)
            draw_text(d, (x + 40, py + 12), path, UI(24), c["path_c"])
            if tag:
                nw = d.textlength(tag, font=UI(22))
                draw_text(d, (x + col_w - 40 - nw, py + 14), tag, UI(22), MUTED)
            py += 60

        py = y + 300
        for line in c["body"]:
            draw_text(d, (x + 28, py), line, UI(24), c["body_c"])
            py += 38

        py = y + 440
        for j, line in enumerate(c["foot"]):
            color = c["foot0"] if j == 0 else (c["foot0"] if i > 0 else DIM)
            draw_text(d, (x + 28, py), line, UI(24), color)
            py += 38
        extra = c.get("extra")
        if extra:
            draw_text(d, (x + 28, py + 6), extra, UI(22), DIM)

    save(img, "layouts.png")


def draw_hooks():
    W, H = 2000, 640
    img, d = new_canvas(W, H)
    pad, gap = 48, 28
    section_label(d, (pad, 36), "双 Hook  ·  开工链窗口，收工做维护", SKY)
    draw_text(d, (pad, 80), "agent 无感：stdout 恒空  ·  日志走 stderr  ·  失败不阻塞会话", UI(26), MUTED)

    steps = [
        (TEAL, SURFACE_TEAL, BORDER_TEAL, "会话开始", "wiki prepare", "已注册项目：建立 / 修复", "项目侧 wiki/ 等窗口链接"),
        (SKY, SURFACE_SKY, BORDER_SKY, "会话中", "路径不变", "agent 仍写 项目/wiki/", "正文落到 WIKI_ROOT/projects/"),
        (AMBER, SURFACE_AMBER, BORDER_AMBER, "会话退出", "wiki check", "维护窗口、迁移正文、", "同步注册表；可自动接入"),
    ]
    col_w = (W - pad * 2 - gap * 2) // 3
    y, h = 140, 440
    for i, (accent, bg, border, kicker, title, l1, l2) in enumerate(steps):
        x = pad + i * (col_w + gap)
        rrect(d, (x, y, x + col_w, y + h), 24, fill=bg, outline=border, width=2)
        d.ellipse((x + 32, y + 40, x + 56, y + 64), fill=accent)
        draw_text(d, (x + 72, y + 36), kicker, UI(26), accent)
        draw_text(d, (x + 32, y + 96), title, BD(40), TEXT)
        draw_text(d, (x + 32, y + 176), l1, UI(26), MUTED)
        draw_text(d, (x + 32, y + 218), l2, UI(26), MUTED)
        if i < 2:
            hline_arrow(d, x + col_w + 2, y + h // 2, x + col_w + gap - 2, accent, width=5, head=12)

    save(img, "hooks.png")


if __name__ == "__main__":
    draw_architecture()
    draw_layouts()
    draw_hooks()
