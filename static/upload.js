var upload = {
    dropbox: null, // HTML upload zone
    stats: null, // HTML upload status
    form: null, // HTML upload form
    init : function () {
        upload.dropbox = document.getElementById("dropbox");
        upload.stats = document.getElementById("statistics");
        upload.form = document.getElementById("odmUpload");
        
        if (window.File && window.FileReader && window.FileList && window.Blob) {
            upload.dropbox.addEventListener("dragenter", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.add('hover');
            });
            upload.dropbox.addEventListener("dragleave", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.remove('hover');
            });
            
            upload.dropbox.addEventListener("dragover", function (e) {
                e.preventDefault();
                e.stopPropagation();
            });
            upload.dropbox.addEventListener("drop", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.remove('hover');
                upload.queue(e.dataTransfer.files);
            });
        }

        else {
            upload.dropbox.style.display = "none";
            upload.form.style.display = "block";
        }
    }
}
window.addEventListener("DOMContentLoaded", upload.init);