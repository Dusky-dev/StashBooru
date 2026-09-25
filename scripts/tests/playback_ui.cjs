// Run Vite in ui/v2.5, then: PLAYWRIGHT_MODULE=/path/to/playwright node scripts/tests/playback_ui.cjs
// No browser or test dependency is added to the production bundle.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(__dirname, '../..');
const baseURL = process.env.PLAYBACK_UI_URL || 'http://127.0.0.1:3042';
const source = ['03', '04'].map(n => fs.readFileSync(path.join(root, `ui/v2.5/builtin-source/unifiedMedia.${n}.part`), 'utf8')).join('');

(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    for (const [name, deps] of [['sync', 'entries'], ['syncNativeLightbox', 'pageVisualEntries']]) {
      const start = `const ${name} = React.useCallback(() => {`;
      const body = source.split(start)[1].split(`}, [${deps}]);`)[0];
      assert.ok(body, `found actual ${name} callback`);
      for (const isScene of [false, true]) {
        await page.setContent(`<div class="Lightbox"><div class="Lightbox-carousel" style="left:0vw"><div class="Lightbox-carousel-image"><div class="Lightbox-carousel-image-wrapper"><img width="64" height="64"></div></div><div class="Lightbox-carousel-image"><div class="Lightbox-carousel-image-wrapper"><img width="64" height="64"></div></div></div><div class="Lightbox-footer-center"><a class="image-link" href="/images/1">Image</a></div></div>`);
        const result = await page.evaluate(async ({ body, isScene, name }) => {
          document.body.className = '';
          let runs = 0, geometryUpdates = 0, pending = 0;
          const entries = isScene ? [{type: 'scene', item: {id:'7'}}, {type:'scene', item:{id:'8'}}] : name === 'sync' ? [] : [{type:'image'}, {type:'image'}];
          const noop = () => {};
          const sync = new Function('entries', 'pageVisualEntries', 'setActiveIndex', 'setPortalTarget', 'setPortalGeometry', 'setActiveVisualIndex', 'setScenePortalTarget', 'setScenePortalGeometry', 'sceneFilename', body).bind(null, entries, entries, noop, noop, () => geometryUpdates++, noop, noop, () => geometryUpdates++, s => `Video ${s.id}`);
          const observer = new MutationObserver(() => {
            if (!pending) pending = requestAnimationFrame(() => {pending = 0; runs++; sync();});
          });
          observer.observe(document.body, {childList:true, subtree:true, attributes:true, attributeFilter:['style','href','class']});
          const frames = async () => { for (let n=0; n<12; n++) await new Promise(requestAnimationFrame); };
          runs++; sync();
          await frames();
          const settledRuns = runs;
          // Navigation and zoom must still wake the observer after it settles.
          document.querySelector('.Lightbox-carousel').style.left = '-100vw';
          document.querySelectorAll('.Lightbox-carousel-image-wrapper')[1].style.transform = 'scale(1.5)';
          await frames();
          observer.disconnect();
          if (pending) cancelAnimationFrame(pending);
          return {settledRuns, afterNavigation:runs, geometryUpdates, href:document.querySelector('.image-link').getAttribute('href')};
        }, {body, isScene, name});
        assert.ok(result.settledRuns <= 2, `${name}/${isScene}: idle observer loop: ${JSON.stringify(result)}`);
        assert.ok(result.afterNavigation > result.settledRuns, 'navigation still synchronizes');
        assert.ok(result.afterNavigation <= 4, 'navigation settles again');
        if (isScene) {
          assert.equal(result.href, '/scenes/8');
          assert.ok(result.geometryUpdates > 1, 'video portal follows geometry changes');
        }
        console.log('Observer settles:', name, isScene ? 'video' : 'image', result);
      }
    }

    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    let submitted;
    const stats = {converted:0,savedBytes:0,averageSavedBytes:0,savedPercent:0,cacheBytes:0,netSavedBytes:0,largerFiles:0};
    const formats = [
      {id:'ajxl', label:'Animated JPEG XL', family:'animation', available:true, cpu:['cjxl'], gpu:[], controls:['quality','effort']},
      {id:'webp', label:'WebP', family:'image', available:true, cpu:['webp'], gpu:[], controls:['quality','lossless']},
      {id:'av1-mp4', label:'AV1 MP4', family:'video', available:true, cpu:['svtav1'], gpu:['av1_nvenc'], controls:['quality']},
      {id:'hevc', label:'HEVC', family:'video', available:false, cpu:[], gpu:[], controls:['quality']},
    ];
    await page.route('**/image/converter*', async route => {
      if (route.request().method() === 'POST') {
        const body = route.request().postDataJSON();
        if (body.action === 'preview') return route.fulfill({json:{plans:[{input:'gif',output:'ajxl',count:1,quality:90,effort:7}]}});
        submitted = body;
        return route.fulfill({json:{}});
      }
      if (route.request().url().includes('capabilities')) return route.fulfill({json:{formats,backend:'local',notice:''}});
      return route.fulfill({json:{config:{backend:'auto',formatDefaults:{gif:'ajxl'},encodingDefaults:{gif:{quality:90,effort:7}},cacheLimitBytes:1024},stats,batchStats:stats,latestStats:stats,history:[],historyTotal:0}});
    });
    await page.route('**/playback-smoke', route => route.fulfill({contentType:'text/html',body:`<div id="root"></div><script type="module">
      import RefreshRuntime from '/@react-refresh';
      RefreshRuntime.injectIntoGlobalHook(window);
      window.$RefreshReg$ = () => {};
      window.$RefreshSig$ = () => (type) => type;
      window.__vite_plugin_react_preamble_installed__ = true;
      import React from '/node_modules/.vite/deps/react.js';
      import ReactDOM from '/node_modules/.vite/deps/react-dom.js';
      import { BrowserRouter } from '/node_modules/.vite/deps/react-router-dom.js';
      window.testReact = {React, ReactDOM, BrowserRouter};
    </script>`}));
    await page.goto(`${baseURL}/playback-smoke`);
    await page.waitForFunction(() => window.testReact);
    await page.evaluate(async () => {
      const {React, ReactDOM, BrowserRouter} = window.testReact;
      const {MediaConversionDialog} = await import('/src/components/Shared/MediaConversionDialog.tsx');
      window.renderConverter = kind => ReactDOM.render(React.createElement(BrowserRouter, {}, React.createElement(MediaConversionDialog, {key:kind, kind, selectedIds:['123'],onHide:()=>{}})), document.getElementById('root'));
      window.renderConverter('image');
    });
    await page.waitForFunction(() => document.querySelector('#converter-format option[value="webp"]'));
    assert.equal(await page.locator('#converter-format').inputValue(), 'auto');
    assert.match(await page.locator('#converter-format option:checked').textContent(), /Animated JPEG XL/);
    await page.selectOption('#converter-format', 'webp');
    await page.getByRole('button', {name:'Convert file', exact:true}).click();
    assert.equal(submitted.options.format, 'webp');
    assert.equal(submitted.useEncodingDefaults, true);
    await page.selectOption('#converter-format', 'auto');
    await page.getByRole('button', {name:'Convert file', exact:true}).click();
    assert.equal(submitted.options.format, 'auto');
    await page.evaluate(() => window.renderConverter('scene'));
    await page.waitForFunction(() => document.querySelector('#converter-format option[value="av1-mp4"]'));
    assert.equal(await page.locator('#converter-format option[value="webp"]').count(), 0);
    assert.equal(await page.locator('#converter-format option[value="hevc"]').isDisabled(), true);
    console.log('Converter: defaults, explicit output payload, scene filtering, unavailable codec passed');
    await page.evaluate(async () => {
      const {React, ReactDOM} = window.testReact;
      const {LightboxImage} = await import('/src/hooks/Lightbox/LightboxImage.tsx');
      const root = document.getElementById('root');
      ReactDOM.unmountComponentAtNode(root);
      const noOp = () => {};
      window.renderSlide = (current, isAnimated) => ReactDOM.render(React.createElement(LightboxImage, {src:'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',width:64,height:64,displayMode:'FIT_XY',scaleUp:true,scrollMode:'ZOOM',zoom:1,scrollAttemptsBeforeChange:1,firstScroll:{current:null},inScrollGroup:{current:false},current,setZoom:noOp,debouncedScrollReset:noOp,onLeft:noOp,onRight:noOp,isVideo:false,isAnimated}),root);
      window.renderSlide(false, true);
    });
    assert.equal(await page.locator('.Lightbox-carousel-image').count(), 1, 'retain carousel slot');
    assert.equal(await page.locator('.Lightbox-carousel-image img').count(), 0, 'hidden animation not decoded');
    await page.evaluate(() => window.renderSlide(true, true));
    assert.equal(await page.locator('.Lightbox-carousel-image img').count(), 1);
    await page.evaluate(() => {window.activeImageNode = document.querySelector('.Lightbox-carousel-image img'); window.renderSlide(true, true);});
    assert.equal(await page.evaluate(() => window.activeImageNode === document.querySelector('.Lightbox-carousel-image img')), true, 'active animation survives rerender');
    await page.evaluate(() => window.renderSlide(false, false));
    assert.equal(await page.locator('.Lightbox-carousel-image img').count(), 1, 'still images preload');
    assert.deepEqual(errors, []);
    console.log('Lightbox: hidden animation suspended, active image stable, still preloading passed');
  } finally {
    await browser.close();
  }
})().catch(e => {console.error(e); process.exitCode = 1;});
