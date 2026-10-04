(() => {
  'use strict';

  const slides = [...document.querySelectorAll('[data-scene]')];
  const visuals = [...document.querySelectorAll('[data-visual]')];
  const chapterButtons = [...document.querySelectorAll('[data-chapter]')];
  const playButton = document.getElementById('play-toggle');
  const replayButton = document.getElementById('replay');
  const scrubber = document.getElementById('scrubber');
  const timecode = document.getElementById('timecode');
  const actIndex = document.getElementById('act-index');
  const endnote = document.getElementById('endnote');
  const core = document.querySelector('.lunar-core');
  const taskRows = [...document.querySelectorAll('.task-row')];
  const taskHeading = document.querySelector('.panel-heading span');
  const taskCount = document.querySelector('.panel-heading strong');
  const taskProgress = document.querySelector('.task-progress span');
  const dispatchSignals = [...document.querySelectorAll('.connection-map path')].map((path, index) => ({
    path, length: path.getTotalLength(), dot: document.querySelectorAll('.dispatch-signal')[index]
  }));
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
  const chapterTimes = [0, 9, 18, 27];
  const totalSeconds = 36;
  let timeline = null;
  let manualPaused = false;
  let hiddenWasPlaying = false;
  let activeChapter = -1;
  let completedTasks = -1;

  function updateDispatch(seconds) {
    dispatchSignals.forEach(({ path, length, dot }, index) => {
      const elapsed = seconds - 9.5 - index * .3;
      const progress = Math.max(0, elapsed % 2.5) / 2.5;
      const point = path.getPointAtLength(length * progress);
      dot.setAttribute('cx', String(point.x));
      dot.setAttribute('cy', String(point.y));
      dot.style.opacity = seconds >= 9.5 && seconds < 18 && elapsed >= 0
        ? String(Math.sin(progress * Math.PI)) : '0';
    });
  }

  function updateTasks(seconds) {
    const completed = seconds >= 25.5 ? 4 : seconds >= 23 ? 3 : seconds >= 20.5 ? 2 : 1;
    if (completed === completedTasks) return;
    completedTasks = completed;
    taskRows.forEach((row, index) => {
      const done = index < completed;
      const active = index === completed;
      row.classList.toggle('done', done);
      row.classList.toggle('active', active);
      row.classList.toggle('queued', !done && !active);
      row.querySelector('.task-check').textContent = done ? '✓' : active ? '↗' : '·';
      row.querySelector('.task-state').textContent = done ? 'Concluída' : active ? 'Em curso' : 'Próxima';
    });
    taskHeading.textContent = completed === 4 ? 'MISSÃO CONCLUÍDA' : 'MISSÃO EM ANDAMENTO';
    taskCount.replaceChildren(document.createTextNode(String(completed).padStart(2, '0') + ' '));
    const total = document.createElement('small');
    total.textContent = '/ 04';
    taskCount.append(total);
    taskProgress.style.transform = `scaleX(${completed / 4})`;
  }

  function formatTime(seconds) {
    return `00:${String(Math.min(totalSeconds, Math.floor(seconds))).padStart(2, '0')}`;
  }

  function setChapter(index) {
    if (activeChapter === index) return;
    activeChapter = index;
    chapterButtons.forEach((button, i) => {
      const current = i === index;
      button.classList.toggle('is-current', current);
      if (current) button.setAttribute('aria-current', 'step');
      else button.removeAttribute('aria-current');
    });
    slides.forEach((slide, i) => {
      slide.setAttribute('aria-hidden', String(i !== index));
      slide.inert = i !== index;
    });
    actIndex.textContent = `${String(index + 1).padStart(2, '0')} / 04`;
  }

  function setPlayLabel(isPlaying) {
    playButton.querySelector('.play-icon').textContent = isPlaying ? 'Ⅱ' : '▶';
    const label = isPlaying ? 'Pausar apresentação' : 'Reproduzir apresentação';
    playButton.setAttribute('aria-label', label);
    playButton.title = label;
  }

  function update() {
    if (!timeline) return;
    const seconds = Math.min(totalSeconds, timeline.time());
    const index = Math.min(3, Math.floor(seconds / 9));
    setChapter(index);
    updateTasks(seconds);
    updateDispatch(seconds);
    scrubber.value = String(Math.round(seconds / totalSeconds * 1000));
    timecode.textContent = `${formatTime(seconds)} / 00:36`;
    const ended = seconds >= totalSeconds - .02;
    endnote.classList.toggle('is-finished', ended);
    endnote.textContent = ended ? 'Sua direção. Um universo de trabalho em movimento.' : 'Dê direção. O Force coordena.';
    setPlayLabel(!timeline.paused() && !ended);
  }

  function showStatic(index) {
    slides.forEach((slide, i) => {
      slide.style.opacity = i === index ? '1' : '0';
      slide.style.visibility = i === index ? 'visible' : 'hidden';
      slide.style.transform = 'none';
    });
    visuals.forEach((scene, i) => {
      scene.style.opacity = i === index ? '1' : '0';
      scene.style.visibility = i === index ? 'visible' : 'hidden';
      scene.style.transform = 'none';
    });
    setChapter(index);
    core.style.setProperty('--core-raised', index >= 2 ? '1' : '0');
    updateTasks(index === 2 ? 22 : chapterTimes[index]);
    scrubber.value = String(Math.round(chapterTimes[index] / totalSeconds * 1000));
    timecode.textContent = `${formatTime(chapterTimes[index])} / 00:36`;
  }

  function seekChapter(index) {
    const safeIndex = Math.max(0, Math.min(3, Number(index) || 0));
    if (!timeline) {
      showStatic(safeIndex);
      return;
    }
    manualPaused = true;
    timeline.pause().time(chapterTimes[safeIndex] + 2.2);
    update();
  }

  function buildTimeline() {
    if (!window.gsap || reducedMotion.matches) return;
    const gsap = window.gsap;
    gsap.set([...slides, ...visuals], { autoAlpha: 0, y: 0 });
    gsap.set([slides[0], visuals[0]], { autoAlpha: 1 });
    gsap.set('.brief-card', { opacity: 0, y: 26 });
    gsap.set('.agent-node', { opacity: 0, y: 18 });
    gsap.set('.connection-map path', { opacity: 0, strokeDashoffset: 90 });
    gsap.set(['.task-panel', '.routine-card'], { opacity: 0, y: 26 });
    gsap.set(core, { '--core-raised': 0 });

    timeline = gsap.timeline({ paused: true, onUpdate: update, onComplete: update });
    // A neutral duration keeps the final chapter visible until its full beat completes.
    const durationMarker = { value: 0 };
    timeline.to(durationMarker, { value: 1, duration: totalSeconds, ease: 'none' }, 0);

    for (let index = 1; index < 4; index += 1) {
      const at = chapterTimes[index];
      timeline.to([slides[index - 1], visuals[index - 1]], {
        autoAlpha: 0, y: -12, duration: .6, ease: 'power2.inOut'
      }, at);
      timeline.fromTo([slides[index], visuals[index]],
        { autoAlpha: 0, y: 16 },
        { autoAlpha: 1, y: 0, duration: .9, ease: 'power2.out', immediateRender: false },
        at + .18);
    }

    // Individual parts enter as the mission becomes more specific.
    timeline.to('.brief-card', { y: 0, opacity: 1, duration: 1.2, ease: 'power3.out' }, .5);
    timeline.to('.agent-node', { y: 0, opacity: 1, stagger: .18, duration: .7, ease: 'power2.out' }, 9.3);
    timeline.to('.connection-map path', { strokeDashoffset: 0, opacity: 1, duration: 1.5, stagger: .13, ease: 'power1.out' }, 9.6);
    timeline.to('.task-panel', { y: 0, opacity: 1, duration: .85, ease: 'power3.out' }, 18.4);
    timeline.to('.routine-card', { y: 0, opacity: 1, duration: .85, ease: 'power3.out' }, 27.4);
    timeline.to(core, { '--core-raised': 1, duration: 1, ease: 'power3.inOut' }, 17.8);
    timeline.to('.lunar-logo', { scale: 1.04, duration: 3, ease: 'sine.inOut' }, 2);
    timeline.to('.lunar-logo', { scale: 1, duration: 3, ease: 'sine.inOut' }, 5);
    timeline.to('.lunar-logo', { scale: 1.045, duration: 3, ease: 'sine.inOut' }, 20);
    timeline.to('.lunar-logo', { scale: 1, duration: 3, ease: 'sine.inOut' }, 23);

    timeline.time(0).play();
    update();
  }

  chapterButtons.forEach((button) => button.addEventListener('click', () => seekChapter(button.dataset.chapter)));

  playButton.addEventListener('click', () => {
    if (!timeline) return;
    if (timeline.time() >= totalSeconds - .02) {
      manualPaused = false;
      timeline.restart();
      update();
      return;
    }
    manualPaused = !timeline.paused();
    if (manualPaused) timeline.pause();
    else timeline.play();
    update();
  });

  replayButton.addEventListener('click', () => {
    if (!timeline) {
      showStatic(0);
      return;
    }
    manualPaused = false;
    timeline.restart();
    update();
  });

  scrubber.addEventListener('input', () => {
    if (!timeline) {
      showStatic(Math.min(3, Math.floor(Number(scrubber.value) / 250)));
      return;
    }
    manualPaused = true;
    timeline.pause().time(Number(scrubber.value) / 1000 * totalSeconds);
    update();
  });

  document.addEventListener('visibilitychange', () => {
    if (!timeline) return;
    if (document.hidden) {
      hiddenWasPlaying = !timeline.paused();
      timeline.pause();
    } else if (hiddenWasPlaying && !manualPaused && timeline.time() < totalSeconds) {
      timeline.play();
      hiddenWasPlaying = false;
    }
    update();
  });

  if (reducedMotion.matches || !window.gsap) {
    showStatic(0);
    playButton.hidden = true;
    endnote.textContent = 'Escolha um capítulo para explorar.';
  } else {
    buildTimeline();
  }

  window.forcePitch = Object.freeze({
    get timeline() { return timeline; },
    seek: seekChapter,
    get chapter() { return activeChapter; }
  });
})();
