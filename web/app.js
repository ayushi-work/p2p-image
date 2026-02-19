// API Configuration
const API_BASE = 'http://localhost:8080';

// State
let selectedFile = null;
let processedImageBlob = null;

// DOM Elements
const uploadBox = document.getElementById('uploadBox');
const fileInput = document.getElementById('fileInput');
const processBtn = document.getElementById('processBtn');
const statusSection = document.getElementById('statusSection');
const statusMessage = document.getElementById('statusMessage');
const progressFill = document.getElementById('progressFill');
const resultsSection = document.getElementById('resultsSection');
const originalImage = document.getElementById('originalImage');
const processedImage = document.getElementById('processedImage');
const downloadBtn = document.getElementById('downloadBtn');
const resetBtn = document.getElementById('resetBtn');
const networkStatus = document.getElementById('networkStatus');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    checkNetworkStatus();
    setInterval(checkNetworkStatus, 5000);
});

function setupEventListeners() {
    // Upload box click
    uploadBox.addEventListener('click', () => fileInput.click());
    
    // File input change
    fileInput.addEventListener('change', handleFileSelect);
    
    // Drag and drop
    uploadBox.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadBox.classList.add('dragover');
    });
    
    uploadBox.addEventListener('dragleave', () => {
        uploadBox.classList.remove('dragover');
    });
    
    uploadBox.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadBox.classList.remove('dragover');
        
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            handleFile(files[0]);
        }
    });
    
    // Process button
    processBtn.addEventListener('click', processImage);
    
    // Download button
    downloadBtn.addEventListener('click', downloadResult);
    
    // Reset button
    resetBtn.addEventListener('click', reset);
}

function handleFileSelect(e) {
    const file = e.target.files[0];
    if (file) {
        handleFile(file);
    }
}

function handleFile(file) {
    // Validate file type
    if (!file.type.startsWith('image/')) {
        alert('Please select an image file');
        return;
    }
    
    // Validate file size (50MB)
    if (file.size > 50 * 1024 * 1024) {
        alert('File size must be less than 50MB');
        return;
    }
    
    selectedFile = file;
    
    // Update UI
    uploadBox.querySelector('.upload-content p').textContent = file.name;
    processBtn.disabled = false;
    
    // Show preview
    const reader = new FileReader();
    reader.onload = (e) => {
        originalImage.src = e.target.result;
    };
    reader.readAsDataURL(file);
}

async function processImage() {
    if (!selectedFile) return;
    
    // Get selected operation
    const operation = document.querySelector('input[name="operation"]:checked').value;
    
    // Show status
    statusSection.style.display = 'block';
    resultsSection.style.display = 'none';
    processBtn.disabled = true;
    
    updateStatus('Uploading image...');
    setProgress(20);
    
    try {
        // Create form data
        const formData = new FormData();
        formData.append('image', selectedFile);
        formData.append('operation', operation);
        
        updateStatus('Requesting consensus from peers...');
        setProgress(40);
        
        // Send request
        const response = await fetch(`${API_BASE}/api/process`, {
            method: 'POST',
            body: formData
        });
        
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Processing failed');
        }
        
        updateStatus('Processing image...');
        setProgress(70);
        
        // Get processed image
        const blob = await response.blob();
        processedImageBlob = blob;
        
        updateStatus('Complete!');
        setProgress(100);
        
        // Show results
        setTimeout(() => {
            statusSection.style.display = 'none';
            resultsSection.style.display = 'block';
            
            const url = URL.createObjectURL(blob);
            processedImage.src = url;
        }, 500);
        
    } catch (error) {
        console.error('Processing error:', error);
        updateStatus(`Error: ${error.message}`);
        setProgress(0);
        processBtn.disabled = false;
    }
}

function updateStatus(message) {
    statusMessage.textContent = message;
}

function setProgress(percent) {
    progressFill.style.width = `${percent}%`;
}

function downloadResult() {
    if (!processedImageBlob) return;
    
    const url = URL.createObjectURL(processedImageBlob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `processed_${Date.now()}.png`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

function reset() {
    selectedFile = null;
    processedImageBlob = null;
    fileInput.value = '';
    
    uploadBox.querySelector('.upload-content p').textContent = 'Click to upload or drag and drop';
    processBtn.disabled = true;
    
    statusSection.style.display = 'none';
    resultsSection.style.display = 'none';
    
    originalImage.src = '';
    processedImage.src = '';
}

async function checkNetworkStatus() {
    try {
        const response = await fetch(`${API_BASE}/api/status`);
        const data = await response.json();
        
        const peerCount = data.peer_count || 0;
        if (peerCount === 0) {
            networkStatus.textContent = 'Local mode';
        } else {
            networkStatus.textContent = `${peerCount} peer${peerCount !== 1 ? 's' : ''}`;
        }
    } catch (error) {
        networkStatus.textContent = 'Offline';
    }
}
