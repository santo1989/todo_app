<?php
include 'db.php';

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $title = $_POST['title'];
    $description = $_POST['description'];
    $status = $_POST['status'];

    if (!empty($title) && !empty($description) && in_array($status, ['todo', 'in-progress', 'done'])) {
        $stmt = $pdo->prepare("INSERT INTO tasks (title, description, status) VALUES (?, ?, ?)");
        $stmt->execute([$title, $description, $status]);
    }
}

header('Location: index.php');
exit;
