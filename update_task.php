<?php
include 'db.php';

$data = json_decode(file_get_contents('php://input'), true);

if (isset($data['id'], $data['status'])) {
    $stmt = $pdo->prepare("UPDATE tasks SET status = ? WHERE id = ?");
    $success = $stmt->execute([$data['status'], $data['id']]);

    echo json_encode(['success' => $success]);
} else {
    echo json_encode(['success' => false]);
}
