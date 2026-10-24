const statuses = ['todo', 'in-progress', 'done']
const board = document.getElementById('board')
const dateInput = document.getElementById('note-date')
const reloadBtn = document.getElementById('reload-btn')
const createForm = document.getElementById('create-form')
const template = document.getElementById('note-template')
const authForm = document.getElementById('auth-form')
const registerBtn = document.getElementById('register-btn')
const logoutBtn = document.getElementById('logout-btn')
const authCard = document.getElementById('auth-card')
const authUser = document.getElementById('auth-user')
const usernameChip = document.getElementById('username-chip')
const feedback = document.getElementById('feedback')

let dragged = null
let currentUser = null

init()

async function init() {
  dateInput.value = todayISO()
  bindDragDrop()
  bindActions()
  await bootstrapAuth()
}

function bindActions() {
  dateInput.addEventListener('change', loadNotes)
  reloadBtn.addEventListener('click', loadNotes)

  createForm.addEventListener('submit', async (event) => {
    event.preventDefault()
    const title = document.getElementById('note-title').value.trim()
    const content = document.getElementById('note-content').value.trim()

    if (!title || !currentUser) {
      return
    }

    const response = await fetch('/api/notes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title,
        content,
        status: 'todo',
        noteDate: dateInput.value,
      }),
    })

    if (!response.ok) {
      showFeedback('Unable to create note.', true)
      return
    }

    createForm.reset()
    showFeedback('Note created.')
    await loadNotes()
  })

  authForm.addEventListener('submit', async (event) => {
    event.preventDefault()
    await authenticate('login')
  })

  registerBtn.addEventListener('click', async () => {
    await authenticate('register')
  })

  logoutBtn.addEventListener('click', async () => {
    await fetch('/api/auth/logout', { method: 'POST' })
    currentUser = null
    updateAuthUI()
    clearBoard()
    showFeedback('Signed out.')
  })
}

function bindDragDrop() {
  board.querySelectorAll('.lane-body').forEach((laneBody) => {
    laneBody.addEventListener('dragover', (event) => {
      event.preventDefault()
      laneBody.classList.add('drag-target')
    })

    laneBody.addEventListener('dragleave', () => {
      laneBody.classList.remove('drag-target')
    })

    laneBody.addEventListener('drop', async (event) => {
      event.preventDefault()
      laneBody.classList.remove('drag-target')

      if (!dragged) {
        return
      }

      laneBody.appendChild(dragged)
      await persistBoardOrder()
      dragged = null
    })
  })
}

function renderNotes(notes) {
  statuses.forEach((status) => {
    const lane = board.querySelector(`.lane-body[data-status="${status}"]`)
    lane.innerHTML = ''
  })

  notes.forEach((note) => {
    const node = template.content.firstElementChild.cloneNode(true)
    node.dataset.id = String(note.id)
    node.dataset.status = note.status
    node.draggable = true

    node.querySelector('.title').textContent = note.title
    node.querySelector('.content').textContent = note.content || 'No details'

    node.addEventListener('dragstart', () => {
      dragged = node
      node.classList.add('dragging')
    })

    node.addEventListener('dragend', () => {
      node.classList.remove('dragging')
    })

    node.querySelector('.edit-btn').addEventListener('click', async () => {
      const nextTitle = window.prompt('Edit title', note.title)
      if (nextTitle === null) {
        return
      }

      const nextContent = window.prompt('Edit details', note.content || '')
      if (nextContent === null) {
        return
      }

      const response = await fetch(`/api/notes/${note.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: nextTitle.trim(),
          content: nextContent.trim(),
        }),
      })

      if (!response.ok) {
        showFeedback('Unable to edit note.', true)
        return
      }

      showFeedback('Note updated.')
      await loadNotes()
    })

    node.querySelector('.delete-btn').addEventListener('click', async () => {
      const response = await fetch(`/api/notes/${note.id}`, { method: 'DELETE' })
      if (!response.ok) {
        showFeedback('Unable to delete note.', true)
        return
      }
      showFeedback('Note deleted.')
      await loadNotes()
    })

    const lane = board.querySelector(`.lane-body[data-status="${note.status}"]`)
    lane.appendChild(node)
  })

  updateCounters()
}

async function loadNotes() {
  if (!currentUser) {
    clearBoard()
    return
  }

  const date = dateInput.value || todayISO()
  const response = await fetch(`/api/notes?date=${encodeURIComponent(date)}`)
  if (!response.ok) {
    if (response.status === 401) {
      currentUser = null
      updateAuthUI()
      clearBoard()
      showFeedback('Your session expired. Login again.', true)
    }
    return
  }

  const payload = await response.json()
  renderNotes(payload.notes || [])
}

function updateCounters() {
  statuses.forEach((status) => {
    const count = board.querySelectorAll(
      `.lane-body[data-status="${status}"] .note`,
    ).length
    document.getElementById(`count-${status}`).textContent = String(count)
  })
}

async function persistBoardOrder() {
  if (!currentUser) {
    return
  }

  const items = []

  statuses.forEach((status) => {
    const cards = board.querySelectorAll(
      `.lane-body[data-status="${status}"] .note`,
    )
    cards.forEach((card, sortOrder) => {
      card.dataset.status = status
      items.push({
        id: Number(card.dataset.id),
        status,
        sortOrder,
      })
    })
  })

  const response = await fetch('/api/notes/reorder', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  })

  if (!response.ok) {
    showFeedback('Unable to save board order.', true)
    return
  }

  updateCounters()
}

async function bootstrapAuth() {
  const response = await fetch('/api/auth/me')
  if (!response.ok) {
    currentUser = null
    updateAuthUI()
    clearBoard()
    return
  }

  const payload = await response.json()
  currentUser = payload.authenticated ? payload.user : null
  updateAuthUI()

  if (currentUser) {
    await loadNotes()
  } else {
    clearBoard()
  }
}

async function authenticate(mode) {
  const username = document.getElementById('auth-username').value.trim()
  const password = document.getElementById('auth-password').value

  if (!username || !password) {
    showFeedback('Username and password are required.', true)
    return
  }

  const endpoint = mode === 'register' ? '/api/auth/register' : '/api/auth/login'
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })

  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    showFeedback(payload.error || 'Authentication failed.', true)
    return
  }

  const payload = await response.json()
  currentUser = payload.user
  updateAuthUI()
  authForm.reset()
  showFeedback(mode === 'register' ? 'Account created.' : 'Welcome back.')
  await loadNotes()
}

function updateAuthUI() {
  const isAuthed = Boolean(currentUser)
  authCard.classList.toggle('hidden', isAuthed)
  authUser.classList.toggle('hidden', !isAuthed)
  createForm.classList.toggle('disabled', !isAuthed)
  board.classList.toggle('disabled', !isAuthed)
  usernameChip.textContent = isAuthed ? currentUser.username : ''
}

function clearBoard() {
  statuses.forEach((status) => {
    const lane = board.querySelector(`.lane-body[data-status="${status}"]`)
    lane.innerHTML = ''
  })
  updateCounters()
}

function showFeedback(message, isError = false) {
  feedback.textContent = message
  feedback.classList.toggle('error', isError)
}

function todayISO() {
  const date = new Date()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}
