[...document.querySelectorAll('.cmt_info')]
  .filter(e => e.querySelector('.written_dccon')) // 디시콘만 지우기
  .forEach(e =>
    fetch('https://gall.dcinside.com/board/comment/comment_delete_submit', {
      headers: {
        'content-type': 'application/x-www-form-urlencoded; charset=UTF-8',
        'x-requested-with': 'XMLHttpRequest'
      },
      body: Object.entries({
        ci_t: get_cookie('ci_c'),
        _GALLTYPE_: _GALLERY_TYPE_,
        mode: 'del',
        id: id.value,
        re_no: e.dataset.no,
        re_password: 'dltkdgk98'
      })
        .map(v => v.join('='))
        .join('&'),
      method: 'POST',
    })
      .then(res => res.text())
      .then(body => console.log(`${e.dataset.no}: ${body}`))
      .catch(console.error)
  )
