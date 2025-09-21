#include <windows.h>
#include <math.h>
#include <stdio.h>

#define PI 3.14159265358979323846

// Structure to represent a 3D point
typedef struct {
    float x, y, z;
} Point3D;

// Structure to represent a 2D point
typedef struct {
    int x, y;
} Point2D;

// Function to perform rotation around the X-axis
Point3D rotateX(Point3D p, float angle) {
    Point3D rotated_p;
    rotated_p.x = p.x;
    rotated_p.y = p.y * cos(angle) - p.z * sin(angle);
    rotated_p.z = p.y * sin(angle) + p.z * cos(angle);
    return rotated_p;
}

// Function to perform rotation around the Y-axis
Point3D rotateY(Point3D p, float angle) {
    Point3D rotated_p;
    rotated_p.x = p.x * cos(angle) + p.z * sin(angle);
    rotated_p.y = p.y;
    rotated_p.z = -p.x * sin(angle) + p.z * cos(angle);
    return rotated_p;
}

// Function to perform rotation around the Z-axis
Point3D rotateZ(Point3D p, float angle) {
    Point3D rotated_p;
    rotated_p.x = p.x * cos(angle) - p.y * sin(angle);
    rotated_p.y = p.x * sin(angle) + p.y * cos(angle);
    rotated_p.z = p.z;
    return rotated_p;
}

// Function for perspective projection
Point2D project(Point3D p, float distance) {
    Point2D projected_p;
    float z_inv = 1 / (distance - p.z);
    projected_p.x = (int)(p.x * z_inv * 500) + 400; // Scaling and centering
    projected_p.y = (int)(p.y * z_inv * 500) + 300; // Scaling and centering
    return projected_p;
}

LRESULT CALLBACK WindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);

int WINAPI WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR pCmdLine, int nCmdShow) {
    const char CLASS_NAME[]  = "CubeWindowClass";

    // Register the window class.
    WNDCLASS wc = {0};
    wc.lpfnWndProc   = WindowProc;
    wc.hInstance     = hInstance;
    wc.lpszClassName = CLASS_NAME;
    wc.hCursor       = LoadCursor(NULL, IDC_ARROW);

    if (!RegisterClass(&wc)) {
        MessageBox(NULL, "Window registration failed!", "Error", MB_ICONEXCLAMATION | MB_OK);
        return 0;
    }

    // Create the window.
    HWND hwnd = CreateWindowEx(
        0,                            // Optional window styles.
        CLASS_NAME,                   // Window class
        "Rotating Cube",              // Window text
        WS_OVERLAPPEDWINDOW,            // Window style

        // Size and position
        CW_USEDEFAULT, CW_USEDEFAULT, 800, 600,

        NULL,       // Parent window    
        NULL,       // Menu
        hInstance,  // Instance handle
        NULL        // Additional application data
        );

    if (hwnd == NULL) {
        MessageBox(NULL, "Window creation failed!", "Error", MB_ICONEXCLAMATION | MB_OK);
        return 0;
    }

    ShowWindow(hwnd, nCmdShow);

    // Run the message loop.
    MSG msg;
    while (GetMessage(&msg, NULL, 0, 0)) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    return 0;
}

LRESULT CALLBACK WindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam) {
    static float angleX = 0, angleY = 0, angleZ = 0;
    static HBITMAP hbmBuffer = NULL;
    static HDC hdcBuffer = NULL;
    static int width, height;

    switch (uMsg) {
        case WM_CREATE: {
            // Initialize double buffering
            RECT rect;
            GetClientRect(hwnd, &rect);
            width = rect.right - rect.left;
            height = rect.bottom - rect.top;

            HDC hdc = GetDC(hwnd);
            hdcBuffer = CreateCompatibleDC(hdc);
            hbmBuffer = CreateCompatibleBitmap(hdc, width, height);
            SelectObject(hdcBuffer, hbmBuffer);
            ReleaseDC(hwnd, hdc);
            return 0;
        }

        case WM_SIZE: {
             //Re-initialize double buffering on window resize
            RECT rect;
            GetClientRect(hwnd, &rect);
            width = rect.right - rect.left;
            height = rect.bottom - rect.top;

            DeleteObject(hbmBuffer);
            DeleteDC(hdcBuffer);

            HDC hdc = GetDC(hwnd);
            hdcBuffer = CreateCompatibleDC(hdc);
            hbmBuffer = CreateCompatibleBitmap(hdc, width, height);
            SelectObject(hdcBuffer, hbmBuffer);
            ReleaseDC(hwnd, hdc);
            return 0;
        }
        case WM_PAINT: {
            PAINTSTRUCT ps;
            HDC hdc = BeginPaint(hwnd, &ps);

            // Draw to the buffer
            SelectObject(hdcBuffer, GetStockObject(BLACK_BRUSH));
            Rectangle(hdcBuffer, 0, 0, width, height);

            // Define the cube vertices
            Point3D cube[] = {
                {-0.5, -0.5, -0.5},
                {0.5, -0.5, -0.5},
                {0.5, 0.5, -0.5},
                {-0.5, 0.5, -0.5},
                {-0.5, -0.5, 0.5},
                {0.5, -0.5, 0.5},
                {0.5, 0.5, 0.5},
                {-0.5, 0.5, 0.5}
            };

            // Rotate the cube
            for (int i = 0; i < 8; i++) {
                cube[i] = rotateX(cube[i], angleX);
                cube[i] = rotateY(cube[i], angleY);
                cube[i] = rotateZ(cube[i], angleZ);
            }

            // Project the cube
            Point2D projected_cube[8];
            for (int i = 0; i < 8; i++) {
                projected_cube[i] = project(cube[i], 3);
            }

            // Draw the cube edges
            HPEN hPen = CreatePen(PS_SOLID, 2, RGB(255, 255, 255)); // White pen
            HPEN hOldPen = (HPEN)SelectObject(hdcBuffer, hPen);

            MoveToEx(hdcBuffer, projected_cube[0].x, projected_cube[0].y, NULL);
            LineTo(hdcBuffer, projected_cube[1].x, projected_cube[1].y);

            MoveToEx(hdcBuffer, projected_cube[1].x, projected_cube[1].y, NULL);
            LineTo(hdcBuffer, projected_cube[2].x, projected_cube[2].y);

            MoveToEx(hdcBuffer, projected_cube[2].x, projected_cube[2].y, NULL);
            LineTo(hdcBuffer, projected_cube[3].x, projected_cube[3].y);

            MoveToEx(hdcBuffer, projected_cube[3].x, projected_cube[3].y, NULL);
            LineTo(hdcBuffer, projected_cube[0].x, projected_cube[0].y);

            MoveToEx(hdcBuffer, projected_cube[4].x, projected_cube[4].y, NULL);
            LineTo(hdcBuffer, projected_cube[5].x, projected_cube[5].y);

            MoveToEx(hdcBuffer, projected_cube[5].x, projected_cube[5].y, NULL);
            LineTo(hdcBuffer, projected_cube[6].x, projected_cube[6].y);

            MoveToEx(hdcBuffer, projected_cube[6].x, projected_cube[6].y, NULL);
            LineTo(hdcBuffer, projected_cube[7].x, projected_cube[7].y);

            MoveToEx(hdcBuffer, projected_cube[7].x, projected_cube[7].y, NULL);
            LineTo(hdcBuffer, projected_cube[4].x, projected_cube[4].y);

            MoveToEx(hdcBuffer, projected_cube[0].x, projected_cube[0].y, NULL);
            LineTo(hdcBuffer, projected_cube[4].x, projected_cube[4].y);

            MoveToEx(hdcBuffer, projected_cube[1].x, projected_cube[1].y, NULL);
            LineTo(hdcBuffer, projected_cube[5].x, projected_cube[5].y);

            MoveToEx(hdcBuffer, projected_cube[2].x, projected_cube[2].y, NULL);
            LineTo(hdcBuffer, projected_cube[6].x, projected_cube[6].y);

            MoveToEx(hdcBuffer, projected_cube[3].x, projected_cube[3].y, NULL);
            LineTo(hdcBuffer, projected_cube[7].x, projected_cube[7].y);

            SelectObject(hdcBuffer, hOldPen);
            DeleteObject(hPen);

            // Blit the buffer to the screen
            BitBlt(hdc, 0, 0, width, height, hdcBuffer, 0, 0, SRCCOPY);
            EndPaint(hwnd, &ps);

            return 0;
        }

        case WM_DESTROY:
            // Clean up GDI objects
            DeleteObject(hbmBuffer);
            DeleteDC(hdcBuffer);
            PostQuitMessage(0);
            return 0;

        case WM_CLOSE:
            DestroyWindow(hwnd);
            return 0;

        default:
            return DefWindowProc(hwnd, uMsg, wParam, lParam);
    }

    // Rotate the angles for the next frame
    angleX += 0.01;
    angleY += 0.02;
    angleZ += 0.03;
    Sleep(16); // Approximately 60 FPS
    InvalidateRect(hwnd, NULL, FALSE); // Trigger a repaint

    return 0;
}
