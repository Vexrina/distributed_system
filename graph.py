import requests
import matplotlib.pyplot as plt
import numpy as np
import re
import time
from datetime import datetime

NUM_OF_NODES = 100

# Устанавливаем backend для работы без GUI
plt.switch_backend('Agg')

def get_data(port: str):
    response = requests.get(f"http://localhost:{port}/value")
    return response.json()

def convert_to_timestamp_method1(time_string):
    # Извлекаем основную часть даты и времени (до +0000 UTC)
    pattern = r'(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+)'
    match = re.search(pattern, time_string)
    
    if match:
        # Преобразуем в datetime объект
        dt = datetime.fromisoformat(match.group(1))
        # Возвращаем timestamp (секунды с начала эпохи)
        return dt.timestamp()
    else:
        raise ValueError("Не удалось извлечь дату из строки")

def get_data_by_experiment():
    arr = np.array([])
    for i in range(NUM_OF_NODES):
        data = get_data(str(8080+i))
        dt = convert_to_timestamp_method1(data['timestamp'])
        arr = np.append(arr, dt)

    # Сортируем timestamps
    arr.sort()
    return arr

def experiment(port: str, num_exp: int, type: str):
    response = requests.post(
        f"http://localhost:{port}/value", 
        json={"value":str(num_exp),"type":type}
    )
    if response.status_code == 200:
        print(response.json())
        print("sleep for 5 sec")
        time.sleep(5)
        return get_data_by_experiment()
    else:
        return None

def test(type: str):
    """
    Выполняет эксперименты и создает матрицу результатов.
    Возвращает средний массив по всем экспериментам.
    """
    # Список для хранения результатов экспериментов
    experiments_results = []
    
    for i in range(NUM_OF_NODES//5):
        arr = experiment(str(8080+i), i, type)
        if arr is not None:
            experiments_results.append(arr)
        else:
            print(f"Эксперимент {i} не удался")
    
    if not experiments_results:
        print("Нет успешных экспериментов")
        return None
    
    # Преобразуем в numpy массив (матрицу)
    # Каждая строка - один эксперимент, каждый столбец - позиция узла
    matrix = np.array(experiments_results)
    
    print(f"Создана матрица размером: {matrix.shape}")
    print(f"Количество экспериментов: {matrix.shape[0]}")
    print(f"Количество узлов: {matrix.shape[1]}")
    
    # Обрабатываем NaN по строкам (экспериментам)
    for row in range(matrix.shape[0]):
        row_data = matrix[row, :]
        # Заменяем NaN на максимум в этой строке (эксперименте)
        max_val = np.nanmax(row_data)
        matrix[row, :] = np.where(np.isnan(row_data), max_val, row_data)
    
    # Вычисляем среднее по столбцам
    mean_arr = np.mean(matrix, axis=0)
    
    return mean_arr

def get_graph(arr: np.ndarray, type):
    """
    Создает график распределения узлов по времени
    """
    if arr is None or len(arr) == 0:
        print("Нет данных для построения графика")
        return
    
    # Создаем массив номеров (индексов) от 0 до len(arr)-1
    node_numbers = np.arange(len(arr))

    # Создаем график
    plt.figure(figsize=(12, 8))
    time_differences = arr - arr[0]
    plt.plot(time_differences, node_numbers, 'r-o', markersize=4, linewidth=1)
    plt.xlabel('Разность времени (миллисекунды)')
    plt.ylabel('Номер узла')
    plt.title('Распределение узлов по времени')
    plt.grid(True, alpha=0.3)

    plt.tight_layout()

    filename = f'{type}_0_procent.png'
    # Сохраняем график в файл вместо показа на экране
    plt.savefig(filename, dpi=300, bbox_inches='tight')
    print("График сохранен в файл 'node_distribution_graph.png'")

    # Выводим статистику
    print(f"Всего узлов: {len(arr)}")
    print(f"Минимальный timestamp: {arr[0]}")
    print(f"Максимальный timestamp: {arr[-1]}")
    print(f"Общий диапазон времени: {arr[-1] - arr[0]:.6f} секунд")
    print(f"Средний интервал между узлами: {(arr[-1] - arr[0]) / (len(arr) - 1):.6f} секунд")

# Пример использования
if __name__ == "__main__":
    experiments = [
        # 'single_cast',
        # 'broadcast',
        # 'multicast',
        'gossip',
    ]
    for exp in experiments:
        # Выполняем эксперименты
        result = test(exp)
        
        if result is not None:
            # Строим график
            get_graph(result, exp)
        else:
            print("Не удалось получить результаты экспериментов")
        
