let intervalId = undefined;
let counter = 0;
const showNotification = () => {
  console.log("=== Showing notification from Service Worker");
  self.registration.showNotification("Hello from Service Worker!", {
    body: "This is a desktop notification.",
    icon: "http://localhost:54349//unlock.png",
    actions: [
      {
        action: "open_app",
        title: "Open App",
      },
    ],
    data: {
      url: "http://localhost:3000", // URL to open when button is clicked
    },
  });
};

const fetchNotificationCounter = async () => {
  const rawResponse = await fetch("/noti.json", {
    method: "GET",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
  });
  const content = await rawResponse.json();
  if (content[0] != counter) {
    counter = content[0];
    showNotification();
  }
};

self.addEventListener("install", (event) => {
  console.log("=== Service Worker install");
  self.skipWaiting(); // Activate worker immediately
});

self.addEventListener("activate", (event) => {
  console.log("=== Service Worker activated");
});

self.addEventListener("message", (event) => {
  if (event.data === "show-notification") {
    showNotification();
  }
  if (event.data === "start-notification") {
    console.log("=== Service Worker start-notification");
    if (intervalId !== undefined) {
      console.log("===clear interval");
      clearInterval(intervalId);
    }
    intervalId = setInterval(() => {
      // fetchNotificationCounter();
    }, 2000);
  }
});

self.addEventListener("notificationclick", async (event) => {
  if (event.action === "open_app") {
    // event.waitUntil(
    //  p =new Promise(async (res, rej) => {
    const rawResponse = await fetch("/noti.json", {
      method: "GET",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
    });
    const content = await rawResponse.json();
    console.log("=== Notification content:", content[0]);
    // await clients.openWindow(event.notification.data.url);
    // event.notification.close();
    //   res();
    // })
    // );
  } else {
    // Handle default click (outside button)
    event.waitUntil(clients.openWindow(event.notification.data.url));
  }
});
