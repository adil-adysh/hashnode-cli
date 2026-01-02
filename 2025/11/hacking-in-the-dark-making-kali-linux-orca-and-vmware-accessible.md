---
title: 'Hacking in the Dark: Making Kali Linux, Orca, and VMware Accessible'
subtitle: ""
slug: hacking-in-the-dark-making-kali-linux-orca-and-vmware-accessible
tags:
    - '@gnome-orca'
    - Kali Linux
    - vmware
    - Accessibility
    - Linux
canonical: ""
cover_image_url: ""
cover_image_attribution: ""
cover_image_photographer: ""
cover_image_stick_bottom: false
cover_image_hide_attribution: false
banner_image_url: ""
disable_comments: null
published_at: 2025-11-23T06:22:52.929Z
meta_title: ""
meta_description: ""
meta_image: ""
publish_as: ""
co_authors: []
series: ""
toc: null
newsletter: null
delisted: null
scheduled: null
slug_overridden: null
pin_to_blog: null
---

# Hacking in the Dark: Making Kali Linux, Orca, and VMware Accessible

**A blind software engineer’s guide to configuring a functional Kali Linux pentesting lab on Windows 11 — using NVDA OCR, SSH, GNOME Terminal, and PipeWire audio adjustments.**

## 👋 Introduction

Running Kali Linux inside a virtual machine is straightforward for sighted users, but it presents specific hurdles for blind users:

* **Silent Boot:** Boot screens offer no audio feedback.
    
* **Desktop Environment:** XFCE (Kali’s default) requires configuration to work well with screen readers.
    
* **Audio Latency:** Virtualized audio often stutters, making the Orca screen reader difficult to understand.
    
* **Terminal Accessibility:** The default terminal emulator lacks full accessibility support.
    

This guide documents a reliable workflow to set up Kali Linux on Windows 11 using VMware. It utilizes NVDA OCR for initial setup, SSH for configuration, and a specific audio tuning configuration to stabilize Orca.

---

## Phase 1 — Platform Evaluation

I evaluated the common methods for running Kali on Windows to find the most accessible option.

### 1\. WSL (Windows Subsystem for Linux)

* **Pros:** Fast performance, low overhead.
    
* **Cons:** No native accessible GUI; lacks USB passthrough for hardware adapters.
    
* **Verdict:** Not ideal for full pentesting workflows.
    

### 2\. Dual Boot

* **Pros:** Direct hardware access.
    
* **Cons:** High risk during troubleshooting. If Linux audio fails, you lose access to the screen reader without a host OS to fall back on.
    
* **Verdict:** Difficult to maintain accessibility during failures.
    

### 3\. VMware Workstation (Recommended)

* **Pros:** Supports USB passthrough and snapshots.
    
* **Accessibility:** The interface works well with NVDA’s OCR feature.
    
* **Verdict:** The most reliable compromise for accessibility and features.
    

---

## Phase 2 — The "Silent Start" Challenge

Kali boots into XFCE. By default, Orca does not launch automatically, and the environment provides limited feedback to assistive technology.

**The Strategy:** Use NVDA OCR to navigate the silent graphical interface, switch to a text-based console (TTY) to enable SSH, and perform the setup remotely.

---

## Phase 3 — Navigating via NVDA OCR

Until Orca is active, NVDA’s Optical Character Recognition (OCR) allows you to read the VM screen.

**Core Keyboard Workflow:**

1. **Ctrl + G** — Focus VM (send keystrokes to guest).
    
2. **Ctrl + Alt** — Release VM focus (return to host).
    
3. **NVDA + R** — OCR the current screen.
    

**The Loop:** (Action in VM) → `Ctrl + Alt` → `NVDA + R` → `Ctrl + G` → (Next Action)

### ⭐ Tip 1: Maximize VMware

Before starting, press `Ctrl + Alt + Enter` to toggle full-screen mode.

* **Why:** Larger text and a cleaner layout improve OCR accuracy.
    

### ⭐ Tip 2: Screen Share Fallback

If OCR fails to read a specific prompt during the initial boot, use **WhatsApp Desktop, Zoom, or Microsoft Teams** to share your screen with a sighted assistant.

Sharing your screen directly via the desktop app allows the assistant to see the VM window clearly, avoiding the glare and framing issues that come with pointing a phone camera at your monitor.

---

## Phase 4 — Enable SSH via TTY

Since the desktop is initially silent, we switch to a TTY (text console) to enable remote access.

1. Power on the VM and wait 20–30 seconds.
    
2. Switch to TTY: `Ctrl + Alt + F3`
    
3. Check the screen (`Ctrl + Alt` → `NVDA + R`) to confirm the text login prompt.
    
4. Log in blindly:
    
    * **user:** `kali`
        
    * **pass:** `kali`
        
5. Enable the SSH service:
    
    ```bash
    sudo systemctl enable --now ssh
    ```
    
6. From Windows, identify the VM’s IP address:
    
    ```plaintext
    arp -a
    ```
    
7. SSH into the VM from Windows Terminal:
    
    ```bash
    ssh kali@<vm-ip>
    ```
    

You now have a stable, text-based environment to configure the system.

---

## Phase 5 — Configuration via SSH

Perform the necessary installation and audio configuration here to ensure the GUI works correctly upon reboot.

### 5.1 — Install GNOME Terminal

The default XFCE terminal has accessibility limitations. GNOME Terminal is a more accessible alternative.

```bash
sudo apt update
sudo apt install -y gnome-terminal
```

### 5.2 — Set Defaults & Bind Shortcut (Ctrl+Alt+T)

We need to ensure that XFCE uses the accessible terminal by default and that the standard keyboard shortcut launches it.

Run these commands to configure the XFCE settings directly:

**1\. Set system-wide and XFCE defaults:**

```bash
# Set Linux system default
sudo update-alternatives --set x-terminal-emulator /usr/bin/gnome-terminal

# Set XFCE session default
xfconf-query -c xfce4-session -p /general/DefaultXTerm -s "gnome-terminal"
```

**2\. Bind** `Ctrl+Alt+T` to GNOME Terminal:

```bash
# Remove old shortcut (if exists) to prevent conflicts
xfconf-query -c xfce4-keyboard-shortcuts -p "/commands/custom/<Primary><Alt>t" -r

# Set new shortcut
xfconf-query -c xfce4-keyboard-shortcuts -p "/commands/custom/<Primary><Alt>t" -n -t string -s "gnome-terminal"
```

**3\. Verify the shortcut:**

```bash
xfconf-query -c xfce4-keyboard-shortcuts -p "/commands/custom/<Primary><Alt>t"
# Output should be: gnome-terminal
```

### 5.3 — Optimize PipeWire Audio

To prevent Orca from stuttering due to virtualization latency, we adjust the buffer settings in PipeWire and WirePlumber.

**1\. Create config directories:**

```bash
mkdir -p ~/.config/wireplumber/wireplumber.conf.d/
mkdir -p ~/.config/pipewire/pipewire.conf.d/
```

**2\. Configure WirePlumber headroom:**

```bash
cat <<EOF > ~/.config/wireplumber/wireplumber.conf.d/50-vm-headroom.conf
monitor.alsa.rules = [
  {
    matches = [
      { "node.name" = "~alsa_output.*" }
    ],
    actions = {
      update-props = {
        "api.alsa.period-size" = 1024,
        "api.alsa.headroom" = 8192,
        "session.suspend-timeout-seconds" = 0,
        "audio.position" = ["FL", "FR"]
      }
    }
  }
]
EOF
```

**3\. Configure PipeWire quantum limits:**

```bash
cat <<EOF > ~/.config/pipewire/pipewire.conf.d/99-vm-limits.conf
context.properties = {
    default.clock.rate = 48000
    default.clock.min-quantum = 1024
    default.clock.quantum = 1024
    default.clock.max-quantum = 8192
}
EOF
```

**4\. Restart audio services:**

```bash
systemctl --user restart wireplumber pipewire pipewire-pulse
```

---

## Phase 6 — Verify Audio Configuration (SSH)

Before rebooting, verify that the settings have been applied. Note that you will not hear audio yet; you are checking the server parameters.

**1\. Verify buffer quantum:**

```bash
pw-top
```

*Look for:* `QUANT: 1024`

**2\. Verify headroom:**

```bash
pw-dump | grep headroom
```

*Expect output:* `"api.alsa.headroom": 8192`

If these values match, the configuration is active.

---

## Phase 7 — Enable Orca & Log In

Reboot the VM:

```bash
sudo reboot
```

### 7.1 — Focus the VM

Once the VM reboots, bring the VMware window to the front and press **Ctrl + G**.

### 7.2 — Confirm Login Screen

Press **Backspace** repeatedly.

* **LightDM Behavior:** Kali's login manager (LightDM) beeps when the username or password field is empty.
    
* **Troubleshooting:** If there is no beep, press **Ctrl + G** again to ensure focus.
    

### ⭐ 7.3 — Enable Speech at Login (F4)

Immediately press **F4**. On LightDM, this launches the Orca screen reader. You should hear speech feedback for the username and password fields.

### 7.4 — Log In

Enter your credentials (`kali` / `kali`).

* **Note:** Orca usually terminates as the session transitions from the login manager to the desktop. You will likely face a few seconds of silence while the XFCE desktop loads.
    

### 7.5 — Re-Enable Accessibility on Desktop

Once the disk activity settles (approx. 10-15 seconds), you need to manually start Orca for your user session.

**1\. Launch Orca via Run Dialog:**

* Press `Alt + F2` to open the Application Finder (Run dialog).
    
* Type: `orca`
    
* Press **Enter**.
    
* *Result:* You should hear "Screen reader on."
    

**2\. Launch GNOME Terminal (Fallback Method):**

* If `Ctrl + Alt + T` does not work immediately, use the Run dialog again.
    
* Press `Alt + F2`.
    
* Type: `gnome-terminal`
    
* Press **Enter**.
    

### 7.6 — Using the Application Menu (`Alt + F1`)

You can also launch applications using the standard Windows-style start menu (Whisker Menu), but there is a catch.

* Press `Alt + F1` to open the menu.
    
* **Accessibility Warning:** The search results in the XFCE menu are not fully accessible; Orca may not announce which item is currently selected in the list.
    
* **Workaround:** When searching, type the **complete name** of the application (e.g., "Firefox" instead of "Fire") and press **Enter** immediately. Relying on arrow keys to navigate the search results is inconsistent.
    

---

## Phase 8 — Audio Stability Test

To confirm the audio allows for sustained speech without artifacts, run a speaker test in the terminal:

```bash
speaker-test -t wav -c 2 -l 1
```

**Listen for:** Clear "Front Left" and "Front Right" audio without popping or stuttering. If this test passes, Orca should remain stable during use.

---

## Conclusion

While cybersecurity tools are not always designed with accessibility in mind, a functional lab is achievable. By combining NVDA OCR for the initial setup, SSH for configuration, and PipeWire adjustments for stability, you can create a usable Kali Linux environment on Windows without needing sighted assistance.

### What's Next?

I am currently testing other desktop environments to find the optimal workflow for blind developers. In future posts, I plan to cover:

* **Desktop Showdown (GNOME 4 vs. MATE):**
    
    * **The New Contender:** I’ve heard promising things about **GNOME 4** and its migration to **GTK4**. Rumor has it that Orca interaction has improved significantly since I last tried it, particularly with responsiveness and semantic navigation.
        
    * **The Old Standard:** I will compare this against **MATE**, which used to be the most accessible desktop environment for Linux. While it was the reliable "gold standard" for years, it always had some annoying quirks that users had to work around.
        
    * **The Goal:** To see if modern GNOME has finally surpassed MATE as the daily driver for blind engineers.
        

**Questions or suggestions?** If you have a preferred Linux setup or specific accessibility challenges you want me to test, let me know in the comments below.