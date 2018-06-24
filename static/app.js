(function () {

  const STORAGE_KEY = "IG"
  const container = document.createElement("div")
  container.style.position = "fixed"
  container.style.width = "200px"
  container.style.bottom = "40px"
  container.style.right = "10px"
  container.style.padding = "1rem"
  container.style.backgroundColor = "whitesmoke"

  const display = document.createElement("div")

  const toggler = document.createElement("button")
  toggler.textContent = "toggle"
  toggler.style.position = "fixed"
  toggler.style.bottom = '10px'
  toggler.style.right = '10px'
  toggler.onclick = () => {
    const display = container.style.display === 'none'
    Store.data.display = display
    Store.save()
    container.style.display = display ? 'block' : 'none'
  }
  const newUserInput = document.createElement("input")
  newUserInput.placeholder = "Enter username + enter"
  newUserInput.onkeypress = (e) => {
    if (e.key !== "Enter") return
    Store.data.users.push(newUserInput.value)
    newUserInput.value = ""
    Store.save()
    render()
  }
  container.appendChild(display)
  container.appendChild(newUserInput)

  document.body.appendChild(container)
  document.body.appendChild(toggler)

  function render() {
    display.innerHTML = getMarkup()
  }

  function getMarkup() {
    let m = `<p><strong>users</strong></p>`
    m += `<ul style="list-style: none; padding-left: 0">`
    Store.data.users.sort().forEach(u => {
      m += `<li><span class="remove-user" style="font-weight: 700; color: darkred; margin-right: 6px; cursor:pointer;" 
onclick="removeUser('${u}')">x</span><a href="/user/${u}">${u}</a></li>`
    })
    m += `</ul>`
    return m
  }

  window.removeUser = (u) => {
    Store.data.users.splice(Store.data.users.indexOf(u), 1)
    Store.save()
    render()
  }

  const Store = {
    data: {users: [], posts: [], display: true},
    load() {
      const d = localStorage.getItem(STORAGE_KEY)
      if (!d) return
      Store.data = JSON.parse(d)
    },
    save() {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(Store.data))
    }
  }

  Store.load()
  if (!Store.data.display) container.style.display = 'none'
  render()

})()