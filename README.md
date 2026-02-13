# Replay

<img width="266" height="115" alt="image" src="https://github.com/user-attachments/assets/a1b0f8c8-db43-4527-a59b-debd5e5bea05" />

Replay is a high-performance system audio recorder and player written in Go. It captures system audio output (loopback), compresses it in real-time using a hybrid Opus+Zstd pipeline, and provides a custom OpenGL-based GUI for managing playback segments.

*(Note: Replace this with an actual screenshot of your UI)*

## 🚀 Key Features

* **System Loopback Recording:** Automatically routes system audio (what you hear) to the recorder via PulseAudio monitor sources.
* **High-Efficiency Compression:** Implements a two-stage compression pipeline:
* **Opus:** For perceptual audio coding.
* **Zstd:** For additional lossless compression of the bitstream.


* **Custom Audio Engine:**
* Low-latency I/O using PortAudio.
* Lock-free Ring Buffers using atomic operations for thread-safe data transfer.


* **Hardware Accelerated UI:**
* Built from scratch using **OpenGL 4.1 Core**.
* Custom shader pipeline (GLSL) for rendering UI elements.
* No heavy UI frameworks — pure vertices and textures.



## 🛠 Tech Stack

* **Language:** Go (Golang)
* **Audio I/O:** PortAudio (`gordonklaus/portaudio`)
* **Graphics:** OpenGL 4.1 (`go-gl`), GLFW
* **Compression:** Opus (`hraban/opus`), Zstd (`klauspost/compress/zstd`)
* **System Integration:** PulseAudio (`pactl` CLI wrappers for routing)

## 🧠 Architecture Highlights

### The Audio Pipeline

The application avoids GC pauses and blocking operations in the audio callback thread by utilizing a custom **Ring Buffer** implementation (`internal/buffer`).

1. **Capture:** Audio frames are captured via PortAudio.
2. **Buffering:** Raw PCM data is written to a lock-free Ring Buffer (`buffer.go`) using `sync/atomic` to manage read/write pointers safe across goroutines.
3. **Processing:** A separate goroutine drains the buffer, encodes audio chunks into Opus frames, and wraps them in Zstd for storage efficiency.
4. **Storage:** Data is written to disk in a custom binary format capable of handling multiple audio segments within a single file.

### The Rendering Engine

Instead of using standard widgets, the UI is rendered directly via the GPU:

* **Shaders:** Custom Vertex and Fragment shaders handle texture mapping and color blending (`shaders.go`).
* **Event Loop:** Integrated with GLFW for handling mouse inputs and window events, manually mapping cursor coordinates to normalized device coordinates (NDC) for UI interaction.

## 📦 Usage

### Prerequisites

* **Linux** (Due to `pactl` dependency for loopback routing)
* **OpenGL** drivers

### Running

**GUI Mode (Default):**

```bash
./replay --path=session.rep

```

**CLI Mode:**

```bash
# Record strictly via CLI
./replay --path=music.rep --mode=record

# Replay strictly via CLI
./replay --path=music.rep --mode=replay

```

## 📝 Controls

* **Record:** Start capturing system audio.
* **Stop:** Finalize the current segment.
* **Play/Pause:** Playback the recorded segment.
* **Prev/Next:** Navigate between recorded segments (if multiple recordings exist in one session).

## ⚠️ Disclaimer

This project relies on `pactl` (PulseAudio Control) for automatic monitor source routing. It is designed primarily for Linux environments using PulseAudio or PipeWire-Pulse.

### Licenses
This project is distributed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

Third-party [LICENSES](licenses)
