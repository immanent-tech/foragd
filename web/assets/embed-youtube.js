import htmx from 'htmx.org/dist/htmx.esm'

class EmbedYoutube extends HTMLElement {
  constructor() {
    super()
    this.player = null
    this.apiReady = false
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return [
      'video-id',
      'width',
      'height',
      'autoplay',
      'playsinline',
      'origin',
      'credentialless',
      'allow',
    ]
  }

  connectedCallback() {
    this._render()
    this._loadAPI()
    window.htmx = htmx
    htmx.process(root)
  }

  attributeChangedCallback(name, oldVal, newVal) {
    if (oldVal !== newVal && this.shadowRoot) {
      this._render()
      if (this.apiReady) this._initPlayer()
    }
  }

  get videoId() {
    return this.getAttribute('video-id')
  }

  get width() {
    return this.getAttribute('width') || '640'
  }

  get height() {
    return this.getAttribute('height') || '390'
  }

  get autoplay() {
    return this.hasAttribute('autoplay')
  }

  get playsinline() {
    return this.hasAttribute('playsinline') ? 1 : 0
  }

  get origin() {
    return this.hasAttribute('origin')
  }

  get credentialless() {
    return this.hasAttribute('credentialless')
  }

  get allow() {
    return this.hasAttribute('allow') || 'compute-pressure'
  }

  _render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: inline-block;
        }
        #player-container {
          width: ${this.width}px;
          height: ${this.height}px;
        }
        #player {
          position: absolute;
          top: 0;
          left: 0;
          width: 100%;
          height: 100%;
        }
      </style>
      <div id="player-container">
        <div id="player"></div>
      </div>
    `
  }

  _loadAPI() {
    // If the API is already loaded globally, use it directly
    if (window.YT && window.YT.Player) {
      this.apiReady = true
      this._initPlayer()
      return
    }

    // If another instance already started loading, wait for it
    if (window._ytApiLoading) {
      window._ytApiCallbacks = window._ytApiCallbacks || []
      window._ytApiCallbacks.push(() => this._initPlayer())
      return
    }

    // First instance to load the API
    window._ytApiLoading = true
    window._ytApiCallbacks = window._ytApiCallbacks || []
    window._ytApiCallbacks.push(() => this._initPlayer())

    // Save any existing onYouTubeIframeAPIReady
    const existing = window.onYouTubeIframeAPIReady
    window.onYouTubeIframeAPIReady = () => {
      if (existing) existing()
      ;(window._ytApiCallbacks || []).forEach((cb) => cb())
      window._ytApiCallbacks = []
      window._ytApiLoading = false
    }

    const tag = document.createElement('script')
    tag.src = 'https://www.youtube.com/iframe_api'
    document.head.appendChild(tag)
  }

  _initPlayer() {
    if (!this.videoId) return

    const container = this.shadowRoot.getElementById('player')
    if (!container) return

    this.player = new YT.Player(container, {
      height: this.height,
      width: this.width,
      videoId: this.videoId,
      playerVars: {
        playsinline: this.playsinline,
        autoplay: this.autoplay ? 1 : 0,
        credentialless: this.credentialless ? 1 : 0,
        origin: this.origin,
      },
      events: {
        onReady: (event) => {
          this.dispatchEvent(
            new CustomEvent('yt-ready', { detail: { player: event.target } })
          )
          if (this.autoplay) event.target.playVideo()
        },
        onStateChange: (event) => {
          this.dispatchEvent(
            new CustomEvent('yt-state-change', {
              detail: { state: event.data },
            })
          )
        },
        onError: (event) => {
          this.dispatchEvent(
            new CustomEvent('yt-error', { detail: { code: event.data } })
          )
        },
      },
    })
  }

  // Public API - proxy to underlying YT player
  play() {
    this.player?.playVideo()
  }
  pause() {
    this.player?.pauseVideo()
  }
  stop() {
    this.player?.stopVideo()
  }
  mute() {
    this.player?.mute()
  }
  unmute() {
    this.player?.unMute()
  }
  seekTo(seconds) {
    this.player?.seekTo(seconds, true)
  }

  disconnectedCallback() {
    this.player?.destroy()
    this.player = null
  }
}

customElements.define('embed-youtube', EmbedYoutube)
