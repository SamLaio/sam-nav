(function () {
  const toggleButton = document.getElementById("ambient-player-toggle");
  const dialog = document.getElementById("ambient-player-dialog");
  const closeButton = document.getElementById("ambient-player-close");
  const playButton = document.getElementById("ambient-play-toggle");
  const masterVolumeInput = document.getElementById("ambient-master-volume");
  const soundList = document.getElementById("ambient-sound-list");

  if (!toggleButton || !dialog || !closeButton || !playButton || !masterVolumeInput || !soundList) return;

  const storageKey = "samNav.front.ambientPlayer";
  const fadeDuration = 250;
  const icons = {
    river: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M4 8c2.6-2.2 5.4-2.2 8 0s5.4 2.2 8 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M4 14c2.6-2.2 5.4-2.2 8 0s5.4 2.2 8 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M6 19c2-.9 4-.9 6 0s4 .9 6 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    waves: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M3 16c2.5 0 2.5-2 5-2s2.5 2 5 2 2.5-2 5-2 2.5 2 3 2" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M3 11c2.5 0 2.5-2 5-2s2.5 2 5 2 2.5-2 5-2 2.5 2 3 2" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    wind: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M3 8h11a3 3 0 1 0-3-3" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M3 13h16a3 3 0 1 1-3 3" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M3 18h7" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    "light-rain": '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M6 15a4 4 0 0 1 .7-7.9A5 5 0 0 1 16.4 8H17a3.5 3.5 0 0 1 0 7H6Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="2"></path><path d="M8 19v1M12 18v2M16 19v1" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    thunder: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M6 13a4 4 0 0 1 .8-7.9A5 5 0 0 1 16.4 6H17a3.5 3.5 0 0 1 1.7 6.6" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path><path d="M13 12 9 19h4l-1 4 5-8h-4l1-3Z" fill="currentColor"></path></svg>',
    "rain-on-umbrella": '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M4 12a8 8 0 0 1 16 0H4Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="2"></path><path d="M12 12v6a2 2 0 0 0 4 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M7 5 6 3M12 4V2M17 5l1-2" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    crickets: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><ellipse cx="12" cy="13" rx="4" ry="3" fill="none" stroke="currentColor" stroke-width="2"></ellipse><path d="M8.5 11 5 8M15.5 11 19 8M9 15l-4 3M15 15l4 3M10 9 8 5M14 9l2-4" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><circle cx="10" cy="12" r=".8" fill="currentColor"></circle><circle cx="14" cy="12" r=".8" fill="currentColor"></circle></svg>',
    birds: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M4 14c4-5 8-5 12 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M12 14c2.5-3.5 5-3.5 8 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path><path d="M8 19c2-2.3 4-2.3 6 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
    owl: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M7 6c1.5-2 3.5-2 5-1 1.5-1 3.5-1 5 1 2 2.7 1.2 9.2-5 13-6.2-3.8-7-10.3-5-13Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="2"></path><circle cx="10" cy="10" r="1.4" fill="currentColor"></circle><circle cx="14" cy="10" r="1.4" fill="currentColor"></circle><path d="m12 12-1.2 2h2.4L12 12Z" fill="currentColor"></path></svg>',
    frog: '<svg aria-hidden="true" viewBox="0 0 24 24" focusable="false"><path d="M7 10a5 5 0 0 1 10 0 6 6 0 0 1-10 0Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="2"></path><circle cx="9" cy="8" r="1.5" fill="none" stroke="currentColor" stroke-width="2"></circle><circle cx="15" cy="8" r="1.5" fill="none" stroke="currentColor" stroke-width="2"></circle><path d="M9 14c1.8 1 4.2 1 6 0" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path></svg>',
  };
  const sounds = [
    { id: "river", label: "River", src: "/static/sounds/ambient/river.mp3" },
    { id: "waves", label: "Waves", src: "/static/sounds/ambient/waves.mp3" },
    { id: "wind", label: "Wind", src: "/static/sounds/ambient/wind.mp3" },
    { id: "light-rain", label: "Light Rain", src: "/static/sounds/ambient/light-rain.mp3" },
    { id: "thunder", label: "Thunder", src: "/static/sounds/ambient/thunder.mp3" },
    { id: "rain-on-umbrella", label: "Rain on Umbrella", src: "/static/sounds/ambient/rain-on-umbrella.mp3" },
    { id: "crickets", label: "Crickets", src: "/static/sounds/ambient/crickets.mp3" },
    {
      id: "birds",
      label: "Birds",
      src: "/static/sounds/ambient/birds.mp3",
      randomSources: ["/static/sounds/ambient/birds.mp3"],
      randomPlayback: {
        minDelay: 8000,
        maxDelay: 28000,
        overlapChance: 0,
      },
    },
    {
      id: "owl",
      label: "Owl",
      src: "/static/sounds/ambient/owl.mp3",
      randomSources: ["/static/sounds/ambient/owl.mp3"],
      randomPlayback: {
        minDelay: 12000,
        maxDelay: 38000,
        overlapChance: 0.2,
        minOverlapDelay: 1000,
        maxOverlapDelay: 4200,
      },
    },
    {
      id: "frog",
      label: "Frog",
      src: "/static/sounds/ambient/frog.mp3",
      randomSources: [
        "/static/sounds/ambient/frog.mp3",
        "/static/sounds/ambient/frog.mp3",
        "/static/sounds/ambient/frog.mp3",
        "/static/sounds/ambient/frog-chorus.mp3",
      ],
      randomPlayback: {
        minDelay: 4200,
        maxDelay: 16000,
        overlapChance: 0.32,
        minOverlapDelay: 350,
        maxOverlapDelay: 2800,
      },
    },
  ];

  const defaultState = {
    masterVolume: 0.8,
    tracks: Object.fromEntries(
      sounds.map((sound, index) => [
        sound.id,
        {
          enabled: index === 0,
          volume: 0.5,
        },
      ]),
    ),
  };

  const readState = () => {
    try {
      const saved = JSON.parse(window.localStorage.getItem(storageKey) || "{}");
      return {
        masterVolume: Number.isFinite(saved.masterVolume) ? saved.masterVolume : defaultState.masterVolume,
        tracks: Object.fromEntries(
          sounds.map((sound) => [
            sound.id,
            {
              enabled: Boolean(saved.tracks?.[sound.id]?.enabled ?? defaultState.tracks[sound.id].enabled),
              volume: Number.isFinite(saved.tracks?.[sound.id]?.volume)
                ? saved.tracks[sound.id].volume
                : defaultState.tracks[sound.id].volume,
            },
          ]),
        ),
      };
    } catch (_) {
      return JSON.parse(JSON.stringify(defaultState));
    }
  };

  const state = readState();
  let isPlaying = false;
  const audioById = new Map();
  const fadeById = new Map();
  const randomTrackById = new Map();

  const saveState = () => {
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(state));
    } catch (_) {}
  };

  const clampVolume = (value) => Math.min(1, Math.max(0, Number(value) || 0));

  const setRangeValue = (range, nextValue) => {
    const min = Number(range.min) || 0;
    const max = Number(range.max) || 1;
    const step = Number(range.step) || 0.01;
    const clamped = Math.min(max, Math.max(min, nextValue));
    const stepped = Math.round(clamped / step) * step;
    range.value = String(Number(stepped.toFixed(4)));
  };

  const bindRangeControl = (range, onChange) => {
    let isDragging = false;

    const updateFromClientX = (clientX) => {
      const rect = range.getBoundingClientRect();
      if (rect.width <= 0) return;
      const ratio = clampVolume((clientX - rect.left) / rect.width);
      const min = Number(range.min) || 0;
      const max = Number(range.max) || 1;
      setRangeValue(range, min + (max - min) * ratio);
      onChange();
    };

    const stopDragging = (event) => {
      if (!isDragging) return;
      isDragging = false;
      if (typeof range.releasePointerCapture === "function") range.releasePointerCapture(event.pointerId);
      event.preventDefault();
    };

    range.addEventListener("pointerdown", (event) => {
      if (event.button !== 0) return;
      isDragging = true;
      range.focus();
      if (typeof range.setPointerCapture === "function") range.setPointerCapture(event.pointerId);
      updateFromClientX(event.clientX);
      event.preventDefault();
    });

    range.addEventListener("pointermove", (event) => {
      if (!isDragging) return;
      updateFromClientX(event.clientX);
      event.preventDefault();
    });

    range.addEventListener("pointerup", stopDragging);
    range.addEventListener("pointercancel", stopDragging);

    range.addEventListener("keydown", (event) => {
      const step = Number(range.step) || 0.01;
      const current = Number(range.value) || 0;
      const keyDelta = {
        ArrowLeft: -step,
        ArrowDown: -step,
        ArrowRight: step,
        ArrowUp: step,
        PageDown: -step * 10,
        PageUp: step * 10,
      }[event.key];

      if (typeof keyDelta === "number") {
        setRangeValue(range, current + keyDelta);
        onChange();
        event.preventDefault();
        return;
      }

      if (event.key === "Home") {
        setRangeValue(range, Number(range.min) || 0);
        onChange();
        event.preventDefault();
      }

      if (event.key === "End") {
        setRangeValue(range, Number(range.max) || 1);
        onChange();
        event.preventDefault();
      }
    });
  };

  const targetVolume = (sound) => clampVolume(state.masterVolume) * clampVolume(state.tracks[sound.id].volume);

  const randomBetween = (min, max) => min + Math.random() * (max - min);

  const stopFade = (sound) => {
    const frame = fadeById.get(sound.id);
    if (!frame) return;
    cancelAnimationFrame(frame);
    fadeById.delete(sound.id);
  };

  const fadeAudio = (sound, to, done) => {
    const audio = audioById.get(sound.id);
    if (!audio) return;
    stopFade(sound);

    const from = audio.volume;
    const startedAt = performance.now();
    const tick = (now) => {
      const progress = Math.min(1, (now - startedAt) / fadeDuration);
      audio.volume = from + (to - from) * progress;
      if (progress < 1) {
        fadeById.set(sound.id, requestAnimationFrame(tick));
        return;
      }
      fadeById.delete(sound.id);
      if (typeof done === "function") done();
    };

    fadeById.set(sound.id, requestAnimationFrame(tick));
  };

  const updateTrackVolume = (sound) => {
    const randomTrack = randomTrackById.get(sound.id);
    if (randomTrack) {
      randomTrack.pool.forEach((audio) => {
        audio.volume = targetVolume(sound);
      });
      return;
    }

    const audio = audioById.get(sound.id);
    if (!audio) return;
    stopFade(sound);
    const nextVolume = targetVolume(sound);
    audio.volume = nextVolume;
  };

  const hasEnabledTrack = () => sounds.some((sound) => state.tracks[sound.id].enabled);

  const syncPlayButton = () => {
    playButton.classList.toggle("is-playing", isPlaying);
    playButton.setAttribute("aria-pressed", String(isPlaying));
    playButton.disabled = !hasEnabledTrack();
    toggleButton.classList.toggle("is-playing", isPlaying);
  };

  const syncTrackButton = (sound) => {
    const button = soundList.querySelector(`[data-sound-toggle="${sound.id}"]`);
    if (!button) return;
    button.classList.toggle("is-active", state.tracks[sound.id].enabled);
    button.setAttribute("aria-pressed", String(state.tracks[sound.id].enabled));
  };

  const playRandomEvent = (sound) => {
    const randomTrack = randomTrackById.get(sound.id);
    if (!randomTrack || !isPlaying || !state.tracks[sound.id].enabled) return false;

    const idleAudio = randomTrack.pool.find((audio) => audio.paused);
    const audio = idleAudio || randomTrack.pool[randomTrack.poolIndex % randomTrack.pool.length];
    randomTrack.poolIndex += 1;
    audio.pause();
    if (audio.src) audio.currentTime = 0;
    audio.src = randomTrack.sources[Math.floor(Math.random() * randomTrack.sources.length)];
    audio.volume = targetVolume(sound);
    audio.play().catch(() => {});
    return true;
  };

  const clearRandomTimers = (randomTrack) => {
    if (!randomTrack) return;
    randomTrack.timers.forEach((timer) => clearTimeout(timer));
    randomTrack.timers.clear();
  };

  const scheduleRandomTrack = (sound) => {
    const randomTrack = randomTrackById.get(sound.id);
    if (!randomTrack || !isPlaying || !state.tracks[sound.id].enabled) return;

    const delay = randomBetween(randomTrack.config.minDelay, randomTrack.config.maxDelay);
    const timer = setTimeout(() => {
      randomTrack.timers.delete(timer);
      if (!isPlaying || !state.tracks[sound.id].enabled) return;

      playRandomEvent(sound);
      if (Math.random() < randomTrack.config.overlapChance) {
        const overlapDelay = randomBetween(randomTrack.config.minOverlapDelay, randomTrack.config.maxOverlapDelay);
        const overlapTimer = setTimeout(() => {
          randomTrack.timers.delete(overlapTimer);
          playRandomEvent(sound);
        }, overlapDelay);
        randomTrack.timers.add(overlapTimer);
      }

      scheduleRandomTrack(sound);
    }, delay);
    randomTrack.timers.add(timer);
  };

  const playRandomTrack = async (sound) => {
    const randomTrack = randomTrackById.get(sound.id);
    if (!randomTrack) return false;
    clearRandomTimers(randomTrack);
    const started = playRandomEvent(sound);
    scheduleRandomTrack(sound);
    return started;
  };

  const pauseRandomTrack = (sound) => {
    const randomTrack = randomTrackById.get(sound.id);
    if (!randomTrack) return;
    clearRandomTimers(randomTrack);
    randomTrack.pool.forEach((audio) => {
      audio.pause();
      if (audio.src) audio.currentTime = 0;
      audio.volume = targetVolume(sound);
    });
  };

  const playTrack = async (sound) => {
    if (randomTrackById.has(sound.id)) return playRandomTrack(sound);

    const audio = audioById.get(sound.id);
    if (!audio) return false;
    const nextVolume = targetVolume(sound);

    if (audio.paused) {
      stopFade(sound);
      audio.volume = 0;
      try {
        await audio.play();
        fadeAudio(sound, nextVolume);
        return true;
      } catch (_) {
        audio.pause();
        audio.volume = nextVolume;
        return false;
      }
    }

    fadeAudio(sound, nextVolume);
    return true;
  };

  const pauseTrack = (sound) => {
    if (randomTrackById.has(sound.id)) {
      pauseRandomTrack(sound);
      return;
    }

    const audio = audioById.get(sound.id);
    if (!audio) return;
    fadeAudio(sound, 0, () => {
      if (!isPlaying || !state.tracks[sound.id].enabled) {
        audio.pause();
        audio.volume = targetVolume(sound);
      }
    });
  };

  const playEnabledTracks = async () => {
    const results = await Promise.all(
      sounds.map((sound) => {
        if (state.tracks[sound.id].enabled) return playTrack(sound);
        pauseTrack(sound);
        return Promise.resolve(false);
      }),
    );

    return results.some(Boolean);
  };

  const pauseTracks = () => {
    sounds.forEach(pauseTrack);
  };

  const setPlaying = async (nextPlaying) => {
    if (nextPlaying && !hasEnabledTrack()) {
      state.tracks[sounds[0].id].enabled = true;
      syncTrackButton(sounds[0]);
      saveState();
    }

    if (!nextPlaying) {
      isPlaying = false;
      pauseTracks();
      syncPlayButton();
      return;
    }

    const started = await playEnabledTracks();
    isPlaying = started;
    if (!started) pauseTracks();
    syncPlayButton();
  };

  const renderSoundList = () => {
    sounds.forEach((sound) => {
      if (Array.isArray(sound.randomSources)) {
        randomTrackById.set(sound.id, {
          config: {
            maxDelay: 7200,
            maxOverlapDelay: 2200,
            minDelay: 1800,
            minOverlapDelay: 250,
            overlapChance: 0.35,
            ...sound.randomPlayback,
          },
          pool: Array.from({ length: 4 }, () => {
            const audio = new Audio();
            audio.loop = false;
            audio.preload = "auto";
            return audio;
          }),
          poolIndex: 0,
          sources: sound.randomSources,
          timers: new Set(),
        });
      } else {
        const audio = new Audio(sound.src);
        audio.loop = true;
        audio.preload = "none";
        audioById.set(sound.id, audio);
      }

      const row = document.createElement("div");
      row.className = "ambient-sound-row";

      const button = document.createElement("button");
      button.type = "button";
      button.className = "ambient-sound-toggle";
      button.dataset.soundToggle = sound.id;
      button.setAttribute("aria-label", sound.label);
      button.title = sound.label;
      button.insertAdjacentHTML("afterbegin", icons[sound.id]);
      button.addEventListener("click", () => {
        const enabled = !state.tracks[sound.id].enabled;
        state.tracks[sound.id].enabled = enabled;
        syncTrackButton(sound);
        saveState();
        if (enabled) setPlaying(true);
        else if (!hasEnabledTrack()) setPlaying(false);
        else if (isPlaying) {
          pauseTrack(sound);
        }
      });

      const range = document.createElement("input");
      range.type = "range";
      range.min = "0";
      range.max = "1";
      range.step = "0.01";
      range.value = String(state.tracks[sound.id].volume);
      range.setAttribute("aria-label", `${sound.label} volume`);
      const updateRange = () => {
        state.tracks[sound.id].volume = clampVolume(range.value);
        updateTrackVolume(sound);
        saveState();
      };
      range.addEventListener("input", updateRange);
      bindRangeControl(range, updateRange);

      row.append(button, range);
      soundList.append(row);
      syncTrackButton(sound);
      updateTrackVolume(sound);
    });
  };

  toggleButton.addEventListener("click", () => {
    if (typeof dialog.showModal === "function") dialog.showModal();
  });

  closeButton.addEventListener("click", () => dialog.close());

  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });

  playButton.addEventListener("click", () => setPlaying(!isPlaying));

  masterVolumeInput.value = String(state.masterVolume);
  const updateMasterVolume = () => {
    state.masterVolume = clampVolume(masterVolumeInput.value);
    sounds.forEach(updateTrackVolume);
    saveState();
  };
  masterVolumeInput.addEventListener("input", updateMasterVolume);
  bindRangeControl(masterVolumeInput, updateMasterVolume);

  renderSoundList();
  syncPlayButton();
})();
