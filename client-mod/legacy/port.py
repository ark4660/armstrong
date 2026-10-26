"""Starts a 1.8.9 port of modern (Mojang-named) Armstrong source files.

    python3 client-mod/legacy/port.py client-mod/src/main/java/com/ark4660/armstrong \
        client-mod/legacy/src/main/java/com/ark4660/armstrong gui/HudEditorScreen.java

Only the generic renames (Minecraft -> MinecraftClient, KeyMapping ->
KeyBinding, GuiGraphics -> Gfx, InputConstants -> LWJGL 2 Keyboard...).
Files with //? blocks are refused. What's left (entity fields, camera,
Gson 2.2) is fixed by hand; the build tells you where.
"""
import os
import re
import sys

GENERIC = [
    ('import net.minecraft.client.Minecraft;', 'import net.minecraft.client.MinecraftClient;'),
    ('import net.minecraft.client.KeyMapping;', 'import net.minecraft.client.option.KeyBinding;'),
    ('import net.minecraft.client.gui.GuiGraphicsExtractor;', 'import com.ark4660.armstrong.compat.Gfx;'),
    ('import net.minecraft.client.gui.screens.Screen;', 'import net.minecraft.client.gui.screen.Screen;'),
    ('import com.mojang.blaze3d.platform.InputConstants;', 'import org.lwjgl.input.Keyboard;'),
    ('import com.ark4660.armstrong.compat.Pose;\n', ''),
    ('import net.minecraft.world.phys.Vec3;', 'import net.minecraft.util.math.Vec3d;'),
    (r're:\bVec3\b', 'Vec3d'),
    (r're:\bMinecraft\b(?!Client)', 'MinecraftClient'),
    (r're:\bKeyMapping\b', 'KeyBinding'),
    (r're:\bGuiGraphicsExtractor\b', 'Gfx'),
    ('InputConstants.KEY_BACKSPACE', 'Keyboard.KEY_BACK'),
    ('InputConstants.KEY_LALT', 'Keyboard.KEY_LMENU'),
    ('InputConstants.MOUSE_BUTTON_LEFT', '0'),
    ('InputConstants.UNKNOWN.getValue()', 'Keyboard.KEY_NONE'),
    ('InputConstants.KEY_', 'Keyboard.KEY_'),
    (r're:Pose\.push\((\w+)\)', r'\1.push()'),
    (r're:Pose\.pop\((\w+)\)', r'\1.pop()'),
    (r're:Pose\.translate\((\w+), ', r'\1.translate('),
    (r're:Pose\.scale\((\w+), ', r'\1.scale('),
    (r're:\n\t\tsuper\(Component\.literal\("[^"]*"\)\);', ''),
    ('protected void init()', 'public void init()'),
    (r're:(?<![\w.])minecraft\.', 'client.'),
    ('.consumeClick()', '.wasPressed()'),
    ('.isDown()', '.isPressed()'),
    ('JsonParser.parseString(', 'new JsonParser().parse('),
    ('out.addFirst(', 'out.add(0, '),
    ('import org.slf4j.Logger;', 'import org.apache.logging.log4j.Logger;'),
    ('import org.slf4j.LoggerFactory;', 'import org.apache.logging.log4j.LogManager;'),
    ('LoggerFactory.getLogger(', 'LogManager.getLogger('),
]

def port(src, dst):
    s = open(src).read()
    if '//?' in s:
        sys.exit(f'{src} has Stonecutter blocks: port it by hand')
    for a, b in GENERIC:
        s = re.sub(a[3:], b, s) if a.startswith('re:') else s.replace(a, b)
    s = s.replace('import net.minecraft.network.chat.Component;\n', '') if 'Component.' not in s else s
    open(dst, 'w').write(s)

src, dst = sys.argv[1], sys.argv[2]
for rel in sys.argv[3:]:
    os.makedirs(os.path.dirname(dst + '/' + rel), exist_ok=True)
    port(src + '/' + rel, dst + '/' + rel)
    print('ported', rel)
