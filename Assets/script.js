function allowDrop(event) {
  event.preventDefault()
}

function drag(event) {
  event.dataTransfer.setData('text', event.target.dataset.id)
}

function drop(event) {
  event.preventDefault()
  const taskId = event.dataTransfer.getData('text')
  const newStatus = event.target.getAttribute('data-status')

  fetch('update_task.php', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: taskId, status: newStatus }),
  })
    .then((response) => response.json())
    .then((data) => {
      if (data.success) location.reload()
    })
}
