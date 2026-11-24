(function () {
  'use strict';

  // All dynamic content is inserted with textContent, never innerHTML.
  function el(tag, props, children) {
    var node = document.createElement(tag);
    Object.keys(props || {}).forEach(function (k) {
      if (k === 'text') node.textContent = props[k];
      else node.setAttribute(k, props[k]);
    });
    (children || []).forEach(function (c) { if (c) node.appendChild(c); });
    return node;
  }

  function list(label, items) {
    if (!items || !items.length) return null;
    return el('p', {}, [el('strong', { text: label + ': ' }), document.createTextNode(items.join(', '))]);
  }

  var DEFINITIONS_SHOWN = 3;

  function definitionItem(d, lang) {
    var li = el('li', { text: d.text });
    if (d.translation) li.appendChild(el('div', { class: 'translation', dir: 'auto', lang: lang, text: d.translation }));
    if (d.example) li.appendChild(el('div', { class: 'example', text: d.example }));
    return li;
  }

  function senseBlock(sense, lang) {
    var shown = sense.definitions.slice(0, DEFINITIONS_SHOWN);
    var rest = sense.definitions.slice(DEFINITIONS_SHOWN);

    var block = el('div', { class: 'sense' }, [
      el('div', { class: 'pos', text: sense.partOfSpeech || '' }),
      el('ol', {}, shown.map(function (d) { return definitionItem(d, lang); }))
    ]);

    if (rest.length) {
      var more = el('ol', { start: String(DEFINITIONS_SHOWN + 1) }, rest.map(function (d) { return definitionItem(d, lang); }));
      block.appendChild(el('details', {}, [
        el('summary', { text: rest.length + ' more definition' + (rest.length === 1 ? '' : 's') }),
        more
      ]));
    }

    var syn = list('Synonyms', sense.synonyms);
    var ant = list('Antonyms', sense.antonyms);
    if (syn) block.appendChild(syn);
    if (ant) block.appendChild(ant);
    return block;
  }

  // Provider-supplied URLs are only linked when they are plain http(s).
  function safeUrl(u) {
    try {
      var parsed = new URL(u);
      return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : null;
    } catch (e) { return null; }
  }

  function link(href, text, label) {
    var safe = safeUrl(href);
    if (!safe) return document.createTextNode(text);
    var attrs = { href: safe, target: '_blank', rel: 'noopener noreferrer' };
    if (label) attrs['aria-label'] = label;
    return el('a', attrs, [document.createTextNode(text)]);
  }

  function pronunciations(data) {
    var items = data.pronunciations && data.pronunciations.length
      ? data.pronunciations
      : (data.phonetic ? [{ text: data.phonetic }] : []);
    if (!items.length) return null;

    return el('p', { class: 'pronunciations' }, items.map(function (p) {
      var chip = el('span', { class: 'chip' }, [el('span', { text: p.text })]);
      if (p.audioUrl) chip.appendChild(link(p.audioUrl, '🔊 Listen', 'Listen to ' + p.text));
      return chip;
    }));
  }

  function attribution(a) {
    if (!a || (!a.sourceName && !a.license)) return null;
    var p = el('p', { class: 'attribution muted' }, [document.createTextNode('Source: ')]);
    p.appendChild(a.sourceUrl ? link(a.sourceUrl, a.sourceName || a.sourceUrl) : document.createTextNode(a.sourceName || 'dictionary'));
    if (a.license) {
      p.appendChild(document.createTextNode(' · '));
      p.appendChild(a.licenseUrl ? link(a.licenseUrl, a.license) : document.createTextNode(a.license));
    }
    return p;
  }

  function renderResult(container, data, saveable, query) {
    container.replaceChildren();

    var card = el('div', { class: 'card' }, [el('h3', { text: data.term }), pronunciations(data)]);
    data.senses.forEach(function (sense) { card.appendChild(senseBlock(sense, query.target)); });
    (data.warnings || []).forEach(function (w) { card.appendChild(el('p', { class: 'warning', text: w })); });
    card.appendChild(attribution(data.attribution));
    container.appendChild(card);

    if (saveable) container.appendChild(saveForm(query));
  }

  // A plain HTML form: saving needs no script, and the server validates it.
  function saveForm(query) {
    function hidden(name, value) { return el('input', { type: 'hidden', name: name, value: value }); }
    return el('form', { class: 'save-form', method: 'post', action: '/words' }, [
      hidden('term', query.term), hidden('source', query.source), hidden('target', query.target),
      el('label', {}, [
        document.createTextNode('Context (optional)'),
        el('textarea', { name: 'context', rows: '2', maxlength: '2000', placeholder: 'The sentence where you met this word' })
      ]),
      el('button', { type: 'submit', text: 'Save word' })
    ]);
  }

  function lookup(query, container, saveable) {
    container.replaceChildren(el('p', { class: 'muted', role: 'status', text: 'Looking up…' }));

    var params = new URLSearchParams({ term: query.term, source: query.source, target: query.target });
    fetch('/api/lookup?' + params.toString(), { headers: { Accept: 'application/json' }, credentials: 'same-origin' })
      .then(function (resp) {
        return resp.json().then(function (body) { return { ok: resp.ok, body: body }; });
      })
      .then(function (r) {
        if (!r.ok) throw new Error(r.body.error || 'Lookup failed.');
        renderResult(container, r.body, saveable, query);
      })
      .catch(function (err) {
        container.replaceChildren(el('p', { class: 'error', role: 'alert', text: err.message || 'Lookup failed.' }));
      });
  }

  document.querySelectorAll('form[data-lookup-form]').forEach(function (form) {
    form.addEventListener('submit', function (ev) {
      ev.preventDefault();
      var term = form.elements.term.value.trim();
      if (!term) return;
      lookup(
        { term: term, source: form.elements.source.value, target: form.elements.target.value },
        document.getElementById(form.dataset.result),
        form.hasAttribute('data-saveable')
      );
    });
  });

  document.querySelectorAll('button[data-lookup-button]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      lookup(
        { term: btn.dataset.term, source: btn.dataset.source, target: btn.dataset.target },
        document.getElementById(btn.dataset.result),
        false
      );
    });
  });

  document.querySelectorAll('form[data-confirm]').forEach(function (form) {
    form.addEventListener('submit', function (ev) {
      if (!window.confirm(form.dataset.confirm)) ev.preventDefault();
    });
  });
})();
