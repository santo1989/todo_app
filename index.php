<?php include 'db.php'; ?>

<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TODO App</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css">
    <link rel="stylesheet" href="Assets/style.css">
</head>

<body>
    <div class="container mt-5">
        <h1 class="text-center mb-4">TODO App</h1>

        <!-- Task Creation Form -->
        <div class="mb-4">
            <form action="add_task.php" method="POST" class="row g-3">
                <div class="col-md-4">
                    <input type="text" name="title" class="form-control" placeholder="Task Title" required>
                </div>
                <div class="col-md-4">
                    <textarea name="description" class="form-control" placeholder="Task Description" rows="1" required></textarea>
                </div>
                <div class="col-md-2">
                    <select name="status" class="form-select">
                        <option value="todo">To Do</option>
                        <option value="in-progress">In Progress</option>
                        <option value="done">Done</option>
                    </select>
                </div>
                <div class="col-md-2">
                    <button type="submit" class="btn btn-primary w-100">Add Task</button>
                </div>
            </form>
        </div>

        <div class="container mt-5">
            <h1 class="text-center mb-4">TODO App</h1>
            <div class="row">
                <?php
                $statuses = ['todo' => 'To Do', 'in-progress' => 'In Progress', 'done' => 'Done'];
                foreach ($statuses as $status => $label) {
                    $stmt = $pdo->prepare("SELECT * FROM tasks WHERE status = ?");
                    $stmt->execute([$status]);
                    $tasks = $stmt->fetchAll(PDO::FETCH_ASSOC);
                ?>
                    <div class="col-md-4">
                        <div class="card">
                            <div class="card-header bg-primary text-white"><?= $label ?></div>
                            <div class="card-body" ondrop="drop(event)" ondragover="allowDrop(event)" data-status="<?= $status ?>">
                                <?php foreach ($tasks as $task) { ?>
                                    <div class="card mb-2 task-card" draggable="true" ondragstart="drag(event)" data-id="<?= $task['id'] ?>">
                                        <div class="card-body">
                                            <h5><?= htmlspecialchars($task['title']) ?></h5>
                                            <p><?= htmlspecialchars($task['description']) ?></p>
                                        </div>
                                    </div>
                                <?php } ?>
                            </div>
                        </div>
                    </div>
                <?php } ?>
            </div>
        </div>

        <script src="Assets/script.js"></script>
</body>

</html>