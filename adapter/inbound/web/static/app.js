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

  function renderResult(container, data, saveable, query) {
    container.replaceChildren();

    var head = el('h3', { text: data.term }, []);
    if (data.phonetic) head.appendChild(el('span', { class: 'phonetic', text: data.phonetic }));

    var card = el('div', { class: 'card' }, [head]);
    if (data.translation) {
      card.appendChild(el('p', { class: 'translation', dir: 'auto', lang: query.target, text: '≈ ' + data.translation }));
    }

    data.senses.forEach(function (sense) {
      var defs = el('ol');
      sense.definitions.forEach(function (d) {
        var li = el('li', { text: d.text });
        if (d.translation) li.appendChild(el('div', { class: 'translation', dir: 'auto', lang: query.target, text: d.translation }));
        defs.appendChild(li);
      });
      card.appendChild(el('div', { class: 'sense' }, [
        el('div', { class: 'pos', text: sense.partOfSpeech || '' }),
        defs,
        sense.examples && sense.examples.length ? el('p', { class: 'muted', text: 'Example: ' + sense.examples[0] }) : null,
        list('Synonyms', sense.synonyms),
        list('Antonyms', sense.antonyms)
      ]));
    });

    (data.warnings || []).forEach(function (w) { card.appendChild(el('p', { class: 'warning', text: w })); });
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
